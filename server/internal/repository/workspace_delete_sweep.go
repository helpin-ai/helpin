package repository

import (
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"
)

// workspaceRetainedTables keep their rows when a workspace is deleted.
// Billing records are financial history and keep workspace_id as an archived
// id (no foreign key); the outboxes null workspace_id through their foreign
// keys so pending deliveries still go out; git integrations are
// organization-wide and deactivated separately.
var workspaceRetainedTables = map[string]bool{
	"billing_ai_usage_periods":      true,
	"billing_ai_usage_ledger":       true,
	"billing_ai_usage_reservations": true,
	"billing_ai_usage_settlements":  true,
	"billing_credit_ledger_legacy":  true,
	"customer_io_outbox":            true,
	"product_analytics_outbox":      true,
	"git_integrations":              true,
}

// workspaceSweepMaxDepth bounds foreign-key recursion; real chains are 3 deep.
const workspaceSweepMaxDepth = 8

type workspaceSweepFK struct {
	child, parent         string
	childCols, parentCols []string
}

// sweepWorkspaceRows deletes the workspace's rows from every table with a
// workspace_id column, so tables without a cascading foreign key cannot leave
// orphans. Rows in tables that block those deletes through a foreign key are
// deleted first. PostgreSQL only: it reads the system catalog.
func sweepWorkspaceRows(tx *gorm.DB, workspaceID string) error {
	var tables []string
	if err := tx.Raw(`
		SELECT c.table_name
		  FROM information_schema.columns c
		  JOIN information_schema.tables t
		    ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		 WHERE c.table_schema = current_schema()
		   AND c.column_name = 'workspace_id'
		   AND t.table_type = 'BASE TABLE'
		   AND c.table_name <> 'workspaces'
		 ORDER BY c.table_name`).Scan(&tables).Error; err != nil {
		return fmt.Errorf("list workspace tables: %w", err)
	}
	fks, err := loadWorkspaceSweepFKs(tx)
	if err != nil {
		return err
	}
	for _, table := range tables {
		if workspaceRetainedTables[table] {
			continue
		}
		where := "workspace_id = ?"
		if err := sweepDelete(tx, fks, table, where, []any{workspaceID}, nil); err != nil {
			return err
		}
	}
	return nil
}

// loadWorkspaceSweepFKs returns foreign keys that block a parent delete
// (NO ACTION or RESTRICT). Cascading and nulling keys are left to PostgreSQL.
func loadWorkspaceSweepFKs(tx *gorm.DB) (map[string][]workspaceSweepFK, error) {
	var rows []struct {
		Child, Parent, ChildCols, ParentCols string
	}
	if err := tx.Raw(`
		SELECT child_rel.relname AS child,
		       parent_rel.relname AS parent,
		       (SELECT string_agg(a.attname, ',' ORDER BY k.ord)
		          FROM unnest(con.conkey) WITH ORDINALITY k(attnum, ord)
		          JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum) AS child_cols,
		       (SELECT string_agg(a.attname, ',' ORDER BY k.ord)
		          FROM unnest(con.confkey) WITH ORDINALITY k(attnum, ord)
		          JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum) AS parent_cols
		  FROM pg_constraint con
		  JOIN pg_namespace n ON n.oid = con.connamespace
		  JOIN pg_class child_rel ON child_rel.oid = con.conrelid
		  JOIN pg_class parent_rel ON parent_rel.oid = con.confrelid
		 WHERE con.contype = 'f'
		   AND con.confdeltype IN ('a', 'r')
		   AND n.nspname = current_schema()`).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list blocking foreign keys: %w", err)
	}
	byParent := map[string][]workspaceSweepFK{}
	for _, row := range rows {
		if row.Child == row.Parent {
			continue // one DELETE statement covers rows referencing rows it also deletes
		}
		byParent[row.Parent] = append(byParent[row.Parent], workspaceSweepFK{
			child: row.Child, parent: row.Parent,
			childCols: strings.Split(row.ChildCols, ","), parentCols: strings.Split(row.ParentCols, ","),
		})
	}
	return byParent, nil
}

// sweepDelete deletes rows of table matching where, after first deleting the
// rows of tables whose blocking foreign keys reference them.
func sweepDelete(tx *gorm.DB, fks map[string][]workspaceSweepFK, table, where string, args []any, path []string) error {
	if len(path) >= workspaceSweepMaxDepth {
		return fmt.Errorf("foreign key chain too deep at %s: %s", table, strings.Join(path, " -> "))
	}
	path = append(path, table)
	for _, fk := range fks[table] {
		if workspaceRetainedTables[fk.child] || slices.Contains(path, fk.child) {
			continue // a retained child keeps blocking, as before this sweep
		}
		childWhere := fmt.Sprintf("(%s) IN (SELECT %s FROM %s WHERE %s)",
			quoteIdentList(fk.childCols), quoteIdentList(fk.parentCols), quoteIdent(table), where)
		if err := sweepDelete(tx, fks, fk.child, childWhere, args, path); err != nil {
			return err
		}
	}
	if err := tx.Exec("DELETE FROM "+quoteIdent(table)+" WHERE "+where, args...).Error; err != nil {
		return fmt.Errorf("delete workspace rows from %s: %w", table, err)
	}
	return nil
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteIdentList(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = quoteIdent(name)
	}
	return strings.Join(quoted, ", ")
}
