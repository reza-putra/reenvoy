package reenvoy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func processTest(command string, args []string) *Process {
	p := new(Process)
	p.command, p.args = command, args
	p.Stdout = os.Stdout
	p.StdErr = os.Stderr
	return p
}

func TestReenvoy_Start(t *testing.T) {
	proc := processTest("echo", []string{"hello", "world"})
	proc.Serve()

	require.Equal(t, true, proc.isFinished)
	require.NotEmpty(t, proc.GetPID())
}
