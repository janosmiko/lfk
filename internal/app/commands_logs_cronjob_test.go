package app

import (
	"errors"
	"os"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The third Job here has the latest timestamp but a different owner UID -
// it must never win the pick, proving the ownership filter still runs.
const cronJobFakeJobsList = `
if [ "$1" = "get" ] && [ "$2" = "cronjob" ] && [ "$3" = "my-cron" ]; then
  echo '{"metadata":{"uid":"cron-uid-1"}}'
elif [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
cat <<'EOF'
{"items":[
  {"metadata":{"name":"my-cron-100","uid":"uid-100","creationTimestamp":"2026-08-10T00:00:00Z","ownerReferences":[{"kind":"CronJob","uid":"cron-uid-1"}]}},
  {"metadata":{"name":"my-cron-200","uid":"uid-200","creationTimestamp":"2026-08-11T00:00:00Z","ownerReferences":[{"kind":"CronJob","uid":"cron-uid-1"}]}},
  {"metadata":{"name":"other-cron-999","uid":"uid-999","creationTimestamp":"2026-08-12T00:00:00Z","ownerReferences":[{"kind":"CronJob","uid":"cron-uid-OTHER"}]}}
]}
EOF
fi
`

// cronJobFakeNoJobs answers `get cronjob my-cron` but returns an empty Jobs list.
const cronJobFakeNoJobs = `
if [ "$1" = "get" ] && [ "$2" = "cronjob" ] && [ "$3" = "my-cron" ]; then
  echo '{"metadata":{"uid":"cron-uid-1"}}'
elif [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
  echo '{"items":[]}'
fi
`

// --- resolveCronJobPodSelector / latestOwnedJob: genuine empty result ---

func TestResolveCronJobPodSelector_PicksNewestJobAndPrefixedLabel(t *testing.T) {
	fakeKubectl(t, cronJobFakeJobsList+`
if [ "$1" = "get" ] && [ "$2" = "pods" ]; then
  case "$*" in
    *"-l batch.kubernetes.io/job-name=my-cron-200,batch.kubernetes.io/controller-uid=uid-200"*) echo "pod/my-cron-200-xyz" ;;
  esac
fi
`)
	selector, err := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	require.NoError(t, err)
	assert.Equal(t, "batch.kubernetes.io/job-name=my-cron-200,batch.kubernetes.io/controller-uid=uid-200", selector)
}

func TestResolveCronJobPodSelector_FallsBackToLegacyLabels(t *testing.T) {
	fakeKubectl(t, cronJobFakeJobsList+`
if [ "$1" = "get" ] && [ "$2" = "pods" ]; then
  case "$*" in
    *"-l job-name=my-cron-200,controller-uid=uid-200"*) echo "pod/my-cron-200-xyz" ;;
  esac
fi
`)
	selector, err := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	require.NoError(t, err)
	assert.Equal(t, "job-name=my-cron-200,controller-uid=uid-200", selector)
}

func TestResolveCronJobPodSelector_NoOwnedJobs(t *testing.T) {
	fakeKubectl(t, cronJobFakeNoJobs)
	selector, err := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	require.ErrorIs(t, err, errCronJobNoRuns)
	assert.Empty(t, selector)
}

func TestResolveCronJobPodSelector_JobExistsButPodsCleanedUp(t *testing.T) {
	fakeKubectl(t, cronJobFakeJobsList)
	selector, err := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	require.ErrorIs(t, err, errCronJobNoRuns)
	assert.Empty(t, selector)
}

// --- resolveCronJobPodSelector: real kubectl failures must not read as "no runs yet" ---

func TestResolveCronJobPodSelector_GetCronJobFails_ReturnsRealError(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "cronjob" ]; then
  echo "Error from server (Forbidden): cronjobs.batch is forbidden" >&2
  exit 1
fi
`)
	_, err := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	require.Error(t, err)
	assert.False(t, errors.Is(err, errCronJobNoRuns), "an RBAC-style failure must not collapse into errCronJobNoRuns")
}

func TestResolveCronJobPodSelector_GetJobsFails_ReturnsRealError(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "cronjob" ]; then
  echo '{"metadata":{"uid":"cron-uid-1"}}'
elif [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
  echo "Error from server: etcdserver: request timed out" >&2
  exit 1
fi
`)
	_, err := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	require.Error(t, err)
	assert.False(t, errors.Is(err, errCronJobNoRuns), "an apiserver outage must not collapse into errCronJobNoRuns")
}

func TestResolveCronJobPodSelector_GetPodsFails_ReturnsRealError(t *testing.T) {
	fakeKubectl(t, cronJobFakeJobsList+`
if [ "$1" = "get" ] && [ "$2" = "pods" ]; then
  echo "Unable to connect to the server: dial tcp: i/o timeout" >&2
  exit 1
fi
`)
	_, err := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	require.Error(t, err)
	assert.False(t, errors.Is(err, errCronJobNoRuns), "a pod-list failure must not collapse into errCronJobNoRuns")
}

// The fixture's third Job has the latest creationTimestamp but a different
// owner UID. Deleting the `!owned { continue }` filter makes this test fail:
// that Job would win the pick instead of my-cron-200.
func TestLatestOwnedJob_IgnoresJobOwnedByDifferentCronJob(t *testing.T) {
	fakeKubectl(t, cronJobFakeJobsList)
	jobName, jobUID, err := latestOwnedJob("kubectl", "", "default", "my-cron", "test-ctx")
	require.NoError(t, err)
	assert.Equal(t, "my-cron-200", jobName)
	assert.Equal(t, "uid-200", jobUID)
}

// A Job left behind by a deleted CronJob still carries the OLD CronJob's UID
// in its ownerReferences. A replacement CronJob with the same name but a new
// UID must not inherit that orphaned Job's run.
func TestLatestOwnedJob_OrphanedJobSameNameDifferentUID_NotMatched(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "cronjob" ] && [ "$3" = "my-cron" ]; then
  echo '{"metadata":{"uid":"cron-uid-NEW"}}'
elif [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
  echo '{"items":[{"metadata":{"name":"my-cron-100","uid":"uid-100","creationTimestamp":"2026-08-10T00:00:00Z","ownerReferences":[{"kind":"CronJob","name":"my-cron","uid":"cron-uid-OLD"}]}}]}'
fi
`)
	_, _, err := latestOwnedJob("kubectl", "", "default", "my-cron", "test-ctx")
	assert.ErrorIs(t, err, errCronJobNoRuns, "an orphaned Job's UID must not match the replacement CronJob's UID")
}

// Two Jobs sharing an exact creationTimestamp must resolve deterministically
// (higher name wins) rather than by whatever order the API returned them in.
func TestLatestOwnedJob_TiebreakOnEqualTimestamp(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "cronjob" ] && [ "$3" = "my-cron" ]; then
  echo '{"metadata":{"uid":"cron-uid-1"}}'
elif [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
  echo '{"items":[
    {"metadata":{"name":"my-cron-100","uid":"uid-100","creationTimestamp":"2026-08-11T00:00:00Z","ownerReferences":[{"kind":"CronJob","name":"my-cron","uid":"cron-uid-1"}]}},
    {"metadata":{"name":"my-cron-200","uid":"uid-200","creationTimestamp":"2026-08-11T00:00:00Z","ownerReferences":[{"kind":"CronJob","name":"my-cron","uid":"cron-uid-1"}]}}
  ]}'
fi
`)
	jobName, jobUID, err := latestOwnedJob("kubectl", "", "default", "my-cron", "test-ctx")
	require.NoError(t, err)
	assert.Equal(t, "my-cron-200", jobName)
	assert.Equal(t, "uid-200", jobUID)
}

// --- startLogStream / kind=CronJob ---

func TestStartLogStream_CronJob_UsesNewestJobSelector(t *testing.T) {
	argvPath := fakeKubectl(t, cronJobFakeJobsList+`
if [ "$1" = "get" ] && [ "$2" = "pods" ]; then
  case "$*" in
    *"-l batch.kubernetes.io/job-name=my-cron-200,batch.kubernetes.io/controller-uid=uid-200"*) echo "pod/my-cron-200-xyz" ;;
  esac
elif [ "$1" = "logs" ]; then
  echo "hello from job pod"
fi
`)
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cron", "default", "CronJob", model.ResourceTypeEntry{})

	cmd := m.startLogStream()
	require.NotNil(t, cmd)
	msg := cmd()
	logMsg, ok := msg.(logLineMsg)
	require.True(t, ok)
	assert.False(t, logMsg.done)
	assert.Equal(t, "hello from job pod", logMsg.line)

	argv, err := os.ReadFile(argvPath)
	require.NoError(t, err)
	assert.Contains(t, string(argv), "-l\nbatch.kubernetes.io/job-name=my-cron-200,batch.kubernetes.io/controller-uid=uid-200")
	assert.NotContains(t, string(argv), "cronjob/my-cron")
}

func TestStartLogStream_CronJob_NoRuns_SendsSentinelInsteadOfKubectlLogs(t *testing.T) {
	fakeKubectl(t, cronJobFakeNoJobs)
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cron", "default", "CronJob", model.ResourceTypeEntry{})

	cmd := m.startLogStream()
	require.NotNil(t, cmd)
	msg := cmd()
	logMsg, ok := msg.(logLineMsg)
	require.True(t, ok)
	assert.Equal(t, cronJobNoRunsSentinel+"my-cron", logMsg.line)

	mdl, _ := m.updateLogLine(logMsg)
	rm := mdl.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.Contains(t, rm.statusMessage, "No runs yet for CronJob my-cron")
}

func TestStartLogStream_CronJob_KubectlFailure_SurfacesRealError(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "cronjob" ]; then
  echo "Error from server (Forbidden): cronjobs.batch is forbidden" >&2
  exit 1
fi
`)
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cron", "default", "CronJob", model.ResourceTypeEntry{})

	cmd := m.startLogStream()
	require.NotNil(t, cmd)
	msg := cmd()
	logMsg, ok := msg.(logLineMsg)
	require.True(t, ok)
	assert.NotEqual(t, cronJobNoRunsSentinel+"my-cron", logMsg.line, "a real kubectl failure must not read as the no-runs marker")

	mdl, _ := m.updateLogLine(logMsg)
	rm := mdl.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotContains(t, rm.statusMessage, "No runs yet")
	assert.Contains(t, rm.statusMessage, "Forbidden")
}

// --- fetchOlderLogs / saveAllLogs: no-runs vs real failure ---

func TestFetchOlderLogs_CronJob_NoRuns_ReturnsSentinelError(t *testing.T) {
	fakeKubectl(t, cronJobFakeNoJobs)
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cron", "default", "CronJob", model.ResourceTypeEntry{})
	m.scheduler.StartWorkers()
	defer m.scheduler.StopWorkers()

	cmd := m.fetchOlderLogs()
	require.NotNil(t, cmd)
	msg := cmd()
	historyMsg, ok := msg.(logHistoryMsg)
	require.True(t, ok)
	require.ErrorIs(t, historyMsg.err, errCronJobNoRuns)

	mdl, _ := m.updateLogHistory(historyMsg)
	assert.True(t, mdl.statusMessageErr)
	assert.Contains(t, mdl.statusMessage, "No runs yet for CronJob my-cron")
}

func TestFetchOlderLogs_CronJob_KubectlFailure_SurfacesRealError(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "cronjob" ]; then
  echo "Unable to connect to the server: dial tcp: i/o timeout" >&2
  exit 1
fi
`)
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cron", "default", "CronJob", model.ResourceTypeEntry{})
	m.scheduler.StartWorkers()
	defer m.scheduler.StopWorkers()

	cmd := m.fetchOlderLogs()
	require.NotNil(t, cmd)
	msg := cmd()
	historyMsg, ok := msg.(logHistoryMsg)
	require.True(t, ok)
	require.Error(t, historyMsg.err)
	assert.False(t, errors.Is(historyMsg.err, errCronJobNoRuns))

	mdl, _ := m.updateLogHistory(historyMsg)
	assert.True(t, mdl.statusMessageErr)
	assert.NotContains(t, mdl.statusMessage, "No runs yet")
}

func TestSaveAllLogs_CronJob_NoRuns_ReturnsSentinelError(t *testing.T) {
	fakeKubectl(t, cronJobFakeNoJobs)
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cron", "default", "CronJob", model.ResourceTypeEntry{})
	m.scheduler.StartWorkers()
	defer m.scheduler.StopWorkers()

	cmd := m.saveAllLogs()
	require.NotNil(t, cmd)
	msg := cmd()
	saveMsg, ok := msg.(logSaveAllMsg)
	require.True(t, ok)
	require.ErrorIs(t, saveMsg.err, errCronJobNoRuns)

	mdl, _ := m.updateLogSaveAll(saveMsg)
	rm := mdl.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.Contains(t, rm.statusMessage, "No runs yet for CronJob my-cron")
}

func TestSaveAllLogs_CronJob_KubectlFailure_SurfacesRealError(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "cronjob" ]; then
  echo "Error from server (Forbidden): cronjobs.batch is forbidden" >&2
  exit 1
fi
`)
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cron", "default", "CronJob", model.ResourceTypeEntry{})
	m.scheduler.StartWorkers()
	defer m.scheduler.StopWorkers()

	cmd := m.saveAllLogs()
	require.NotNil(t, cmd)
	msg := cmd()
	saveMsg, ok := msg.(logSaveAllMsg)
	require.True(t, ok)
	require.Error(t, saveMsg.err)
	assert.False(t, errors.Is(saveMsg.err, errCronJobNoRuns))

	mdl, _ := m.updateLogSaveAll(saveMsg)
	rm := mdl.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotContains(t, rm.statusMessage, "No runs yet")
}

// --- background-tab CronJob stream must not leak the sentinel into a log buffer ---

func TestUpdateLogLine_BackgroundTab_CronJobNoRuns_SetsStatusNotBuffer(t *testing.T) {
	bgCh := make(chan string)
	activeCh := make(chan string)
	m := Model{
		mode:      modeLogs,
		width:     80,
		height:    40,
		activeTab: 0,
		tabs: []TabState{
			{}, // active tab
			{logCh: bgCh},
		},
		logView: logViewState{ch: activeCh},
	}

	ret, _ := m.updateLogLine(logLineMsg{line: cronJobNoRunsSentinel + "my-cron", ch: bgCh})
	rm := ret.(Model)

	assert.Empty(t, rm.tabs[1].logLines, "the sentinel must never reach a tab's log buffer")
	assert.True(t, rm.statusMessageErr)
	assert.Contains(t, rm.statusMessage, "No runs yet for CronJob my-cron")
}

func TestUpdateLogLine_BackgroundTab_CronJobError_SetsStatusNotBuffer(t *testing.T) {
	bgCh := make(chan string)
	activeCh := make(chan string)
	m := Model{
		mode:      modeLogs,
		width:     80,
		height:    40,
		activeTab: 0,
		tabs: []TabState{
			{},
			{logCh: bgCh},
		},
		logView: logViewState{ch: activeCh},
	}

	ret, _ := m.updateLogLine(logLineMsg{line: cronJobErrorSentinel + "boom", ch: bgCh})
	rm := ret.(Model)

	assert.Empty(t, rm.tabs[1].logLines, "the sentinel must never reach a tab's log buffer")
	assert.True(t, rm.statusMessageErr)
	assert.Contains(t, rm.statusMessage, "boom")
}

// --- Job (non-CronJob) must keep working through the existing selector path ---

func TestStartLogStream_Job_StillUsesKubectlGetPodSelector(t *testing.T) {
	argvPath := fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "job/my-job" ]; then
  echo '{"spec":{"selector":{"matchLabels":{"batch.kubernetes.io/controller-uid":"job-uid"}}}}'
elif [ "$1" = "logs" ]; then
  echo "job log line"
fi
`)
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-job", "default", "Job", model.ResourceTypeEntry{})

	cmd := m.startLogStream()
	require.NotNil(t, cmd)
	msg := cmd()
	logMsg, ok := msg.(logLineMsg)
	require.True(t, ok)
	assert.Equal(t, "job log line", logMsg.line)

	argv, err := os.ReadFile(argvPath)
	require.NoError(t, err)
	assert.Contains(t, string(argv), "-l\nbatch.kubernetes.io/controller-uid=job-uid")
}
