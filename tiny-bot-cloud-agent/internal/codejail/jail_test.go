package codejail

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFilename(t *testing.T) {
	assert.NoError(t, ValidateFilename("hello.py"))
	assert.NoError(t, ValidateFilename("a_b-1.py"))
	assert.ErrorIs(t, ValidateFilename("../x.py"), ErrBadFilename)
	assert.ErrorIs(t, ValidateFilename("a/b.py"), ErrBadFilename)
	assert.ErrorIs(t, ValidateFilename("x.sh"), ErrBadFilename)
	assert.ErrorIs(t, ValidateFilename(""), ErrBadFilename)
}

func TestWriteAndRun(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	root := t.TempDir()
	j := &Jail{WorkspaceRoot: root, Timeout: 5 * time.Second}
	name, err := j.Write("dev1", "hi.py", "print('hello-jail')\n")
	require.NoError(t, err)
	assert.Equal(t, "hi.py", name)

	res, err := j.Run(context.Background(), "dev1", "hi.py")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 0, res.ExitCode)
	assert.Contains(t, res.Stdout, "hello-jail")
	assert.False(t, res.TimedOut)

	_, err = os.Stat(filepath.Join(root, "scratch", "dev1", "hi.py"))
	require.NoError(t, err)
}

func TestWriteRejectsTraversal(t *testing.T) {
	j := &Jail{WorkspaceRoot: t.TempDir()}
	_, err := j.Write("dev1", "../escape.py", "print(1)\n")
	assert.ErrorIs(t, err, ErrBadFilename)
}

func TestWriteRejectsTooLarge(t *testing.T) {
	j := &Jail{WorkspaceRoot: t.TempDir(), MaxFileBytes: 16}
	_, err := j.Write("dev1", "big.py", strings.Repeat("x", 32))
	assert.ErrorIs(t, err, ErrTooLarge)
}

func TestRunTimeout(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	j := &Jail{WorkspaceRoot: t.TempDir(), Timeout: 200 * time.Millisecond}
	_, err := j.Write("dev1", "sleep.py", "import time\ntime.sleep(5)\n")
	require.NoError(t, err)
	res, err := j.Run(context.Background(), "dev1", "sleep.py")
	require.NoError(t, err)
	assert.True(t, res.TimedOut)
	assert.Equal(t, -1, res.ExitCode)
}

func TestRunMissingPython(t *testing.T) {
	j := &Jail{WorkspaceRoot: t.TempDir(), PythonBin: "python3-definitely-missing-xyz"}
	_, err := j.Write("dev1", "a.py", "print(1)\n")
	require.NoError(t, err)
	_, err = j.Run(context.Background(), "dev1", "a.py")
	assert.ErrorIs(t, err, ErrPythonMissing)
}

func TestRunMissingFile(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	j := &Jail{WorkspaceRoot: t.TempDir()}
	_, err := j.Run(context.Background(), "dev1", "nope.py")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEnvDoesNotInheritSecrets(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	t.Setenv("TB_OPENAI_COMPAT_API_KEY", "secret-should-not-leak")
	j := &Jail{WorkspaceRoot: t.TempDir()}
	_, err := j.Write("dev1", "env.py", "import os\nprint(os.environ.get('TB_OPENAI_COMPAT_API_KEY', 'absent'))\n")
	require.NoError(t, err)
	res, err := j.Run(context.Background(), "dev1", "env.py")
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Contains(t, res.Stdout, "absent")
	assert.NotContains(t, res.Stdout, "secret-should-not-leak")
}
