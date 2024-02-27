package uninstall

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/kolide/launcher/ee/agent"
	"github.com/kolide/launcher/ee/agent/types"
)

// Uninstall just removes the enroll secret file and wipes the database.
// Logs errors, but does not return them, because we want to try each step independently.
// If exitOnCompletion is true, it will also disable launcher autostart and exit.
func Uninstall(ctx context.Context, k types.Knapsack, exitOnCompletion bool) {
	slogger := k.Slogger().With("component", "uninstall")

	if err := removeEnrollSecretFile(k); err != nil {
		slogger.Log(ctx, slog.LevelError,
			"removing enroll secret file",
			"err", err,
		)
	}

	if err := agent.WipeDatabase(ctx, k); err != nil {
		slogger.Log(ctx, slog.LevelError,
			"wiping database",
			"err", err,
		)
	}

	if !exitOnCompletion {
		return
	}

	if err := disableAutoStart(ctx); err != nil {
		k.Slogger().Log(ctx, slog.LevelError,
			"disabling auto start",
			"err", err,
		)
	}

	os.Exit(0)
}

func removeEnrollSecretFile(knapsack types.Knapsack) error {
	if knapsack.EnrollSecretPath() == "" {
		return errors.New("no enroll secret path set")
	}

	if err := os.Remove(knapsack.EnrollSecretPath()); err != nil {
		return err
	}

	return nil
}
