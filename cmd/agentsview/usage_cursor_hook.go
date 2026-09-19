package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.kenn.io/agentsview/internal/config"
	"go.kenn.io/agentsview/internal/cursorhook"
	"go.kenn.io/agentsview/internal/db"
)

func ingestCursorHookUsage(
	ctx context.Context, appCfg config.Config, database *db.DB,
) (int, error) {
	if database == nil {
		return 0, fmt.Errorf("database is required")
	}
	return cursorhook.Ingest(ctx, database, appCfg.DataDir)
}

func logCursorHookIngest(count int, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "cursor hook usage ingest: %v\n", err)
		return
	}
	if count > 0 {
		fmt.Fprintf(os.Stderr, "ingested %d cursor hook usage event(s)\n", count)
	}
}

func newUsageCursorHookCommand() *cobra.Command {
	var backfillSessionIDs bool
	cmd := &cobra.Command{
		Use:          "cursor-hook",
		Short:        "Ingest Cursor hook telemetry from the global store",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			appCfg, err := config.LoadMinimal()
			if err != nil {
				return err
			}
			database, writeLock, err := openWriteDB(
				context.Background(),
				appCfg,
			)
			if err != nil {
				return err
			}
			defer closeWriteDB(database, writeLock)
			count, err := ingestCursorHookUsage(cmd.Context(), appCfg, database)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout,
				"Ingested %d Cursor hook usage event(s) into the archive\n",
				count,
			)
			if backfillSessionIDs {
				updated, err := cursorhook.BackfillSessionIDs(
					cmd.Context(), database, appCfg.DataDir,
				)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stdout,
					"Backfilled session_id on %d existing hook row(s)\n",
					updated,
				)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(
		&backfillSessionIDs, "backfill-session-ids", false,
		"Set session_id on existing hook rows from the JSONL log",
	)
	return cmd
}
