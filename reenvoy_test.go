package reenvoy_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/evo3cx/reenvoy"
)

func TestReenvoy_Start(t *testing.T) {
	opts := reenvoy.SpawnOptions{
		Command:      "echo",
		Args:         []string{"hello", "world"},
		ReloadSignal: os.Interrupt,
		KillSignal:   os.Kill,
		KillTimeout:  2 * time.Second,
	}

	proc, err := reenvoy.SpawnProcess(opts)
	require.Nil(t, err, "start reenvoy")
	require.NotEmpty(t, proc.GetPID())
}
