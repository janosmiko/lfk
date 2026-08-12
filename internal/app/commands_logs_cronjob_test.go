package app

import (
	"os"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cronJobFakeJobsList = `
if [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
cat <<'EOF'
{"items":[
  {"metadata":{"name":"my-cron-100","uid":"uid-100","creationTimestamp":"2026-08-10T00:00:00Z","ownerReferences":[{"kind":"CronJob","name":"my-cron"}]}},
  {"metadata":{"name":"my-cron-200","uid":"uid-200","creationTimestamp":"2026-08-11T00:00:00Z","ownerReferences":[{"kind":"CronJob","name":"my-cron"}]}}
]}
EOF
fi
`

// --- resolveCronJobPodSelector ---

func TestResolveCronJobPodSelector_PicksNewestJobAndPrefixedLabel(t *testing.T) {
	fakeKubectl(t, cronJobFakeJobsList+`
if [ "$1" = "get" ] && [ "$2" = "pods" ]; then
  case "$*" in
    *"-l batch.kubernetes.io/job-name=my-cron-200,batch.kubernetes.io/controller-uid=uid-200"*) echo "pod/my-cron-200-xyz" ;;
  esac
fi
`)
	selector, ok := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	require.True(t, ok)
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
	selector, ok := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	require.True(t, ok)
	assert.Equal(t, "job-name=my-cron-200,controller-uid=uid-200", selector)
}

func TestResolveCronJobPodSelector_NoOwnedJobs(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
  echo '{"items":[]}'
fi
`)
	selector, ok := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	assert.False(t, ok)
	assert.Empty(t, selector)
}

func TestResolveCronJobPodSelector_JobExistsButPodsCleanedUp(t *testing.T) {
	fakeKubectl(t, cronJobFakeJobsList)
	selector, ok := resolveCronJobPodSelector("kubectl", "", "default", "my-cron", "test-ctx")
	assert.False(t, ok)
	assert.Empty(t, selector)
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
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
  echo '{"items":[]}'
fi
`)
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

// --- fetchOlderLogs / saveAllLogs no-runs handling ---

func TestFetchOlderLogs_CronJob_NoRuns_ReturnsSentinelError(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
  echo '{"items":[]}'
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
	require.ErrorIs(t, historyMsg.err, errCronJobNoRuns)

	mdl, _ := m.updateLogHistory(historyMsg)
	assert.True(t, mdl.statusMessageErr)
	assert.Contains(t, mdl.statusMessage, "No runs yet for CronJob my-cron")
}

func TestSaveAllLogs_CronJob_NoRuns_ReturnsSentinelError(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "get" ] && [ "$2" = "jobs" ]; then
  echo '{"items":[]}'
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
	require.ErrorIs(t, saveMsg.err, errCronJobNoRuns)

	mdl, _ := m.updateLogSaveAll(saveMsg)
	rm := mdl.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.Contains(t, rm.statusMessage, "No runs yet for CronJob my-cron")
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
