//go:build linux
// +build linux

package cryptsetup

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/kolide/launcher/ee/allowedcmd"
	"github.com/kolide/launcher/ee/dataflatten"
	"github.com/kolide/launcher/ee/tables/dataflattentable"
	"github.com/kolide/launcher/ee/tables/tablehelpers"
	"github.com/osquery/osquery-go/plugin/table"
)

const allowedNameCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-/_"

type Table struct {
	slogger *slog.Logger
	logger  log.Logger // preserved only for use in dataflattentable/tablehelpers.Exec temporarily
	name    string
}

func TablePlugin(slogger *slog.Logger, logger log.Logger) *table.Plugin {
	columns := dataflattentable.Columns(
		table.TextColumn("name"),
	)

	t := &Table{
		slogger: slogger.With("table", "kolide_cryptsetup_status"),
		logger:  logger,
		name:    "kolide_cryptsetup_status",
	}

	return table.NewPlugin(t.name, columns, t.generate)
}

func (t *Table) generate(ctx context.Context, queryContext table.QueryContext) ([]map[string]string, error) {
	var results []map[string]string

	requestedNames := tablehelpers.GetConstraints(queryContext, "name",
		tablehelpers.WithAllowedCharacters(allowedNameCharacters),
		tablehelpers.WithLogger(t.logger),
	)

	if len(requestedNames) == 0 {
		return results, fmt.Errorf("The %s table requires that you specify a constraint for name", t.name)
	}

	for _, name := range requestedNames {
		output, err := tablehelpers.Exec(ctx, t.logger, 15, allowedcmd.Cryptsetup, []string{"--readonly", "status", name}, false)
		if err != nil {
			t.slogger.Log(ctx, slog.LevelDebug,
				"error execing for status",
				"name", name,
				"err", err,
			)
			continue
		}

		status, err := parseStatus(output)
		if err != nil {
			t.slogger.Log(ctx, slog.LevelInfo,
				"error parsing status",
				"name", name,
				"err", err,
			)
			continue
		}

		for _, dataQuery := range tablehelpers.GetConstraints(queryContext, "query", tablehelpers.WithDefaults("*")) {
			flatData, err := t.flattenOutput(dataQuery, status)
			if err != nil {
				t.slogger.Log(ctx, slog.LevelInfo,
					"flatten failed",
					"err", err,
				)
				continue
			}

			rowData := map[string]string{"name": name}

			results = append(results, dataflattentable.ToMap(flatData, dataQuery, rowData)...)
		}
	}

	return results, nil
}

func (t *Table) flattenOutput(dataQuery string, status map[string]interface{}) ([]dataflatten.Row, error) {
	flattenOpts := []dataflatten.FlattenOpts{
		dataflatten.WithLogger(t.logger),
		dataflatten.WithQuery(strings.Split(dataQuery, "/")),
	}

	return dataflatten.Flatten(status, flattenOpts...)
}
