package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPush2IsContextCanceled(t *testing.T) {
	assert.False(t, isContextCanceled(nil))
	assert.True(t, isContextCanceled(context.Canceled))
	assert.True(t, isContextCanceled(context.DeadlineExceeded))
	assert.True(t, isContextCanceled(fmt.Errorf("context canceled")))
	assert.True(t, isContextCanceled(fmt.Errorf("context deadline exceeded")))
	assert.False(t, isContextCanceled(errors.New("random error")))
}

func TestFinalIsContextCanceled(t *testing.T) {
	assert.False(t, isContextCanceled(nil))
	assert.True(t, isContextCanceled(context.Canceled))
	assert.True(t, isContextCanceled(context.DeadlineExceeded))
	assert.False(t, isContextCanceled(assert.AnError))
}
