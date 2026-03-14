package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

var errTestSimulated = errors.New("simulated error")

func TestNewDB(t *testing.T) {
	t.Run("SuccessfulDBCreation", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.Close()
		require.NoError(t, err)
	})

	t.Run("DatabaseFileCreated", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		_, err = os.Stat(dbPath)
		assert.NoError(t, err)
		assert.True(t, isFile(dbPath))

		err = db.Close()
		require.NoError(t, err)
	})

	t.Run("InvalidPath", func(t *testing.T) {
		// Test with path that doesn't exist
		db, err := New(context.Background(), "/nonexistent/path/test.db")
		assert.Error(t, err)
		assert.Nil(t, db)
	})

	t.Run("EmptyPath", func(t *testing.T) {
		db, err := New(context.Background(), "")
		assert.Error(t, err)
		assert.Nil(t, db)
	})

	t.Run("ValidPathWithPermissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		// Create directory
		err := os.MkdirAll(tmpDir, 0755)
		require.NoError(t, err)

		// Try to create database in directory
		db, err := New(context.Background(), dbPath)
		assert.NoError(t, err)
		assert.NotNil(t, db)

		err = db.Close()
		require.NoError(t, err)
	})

	t.Run("DBPingSuccess", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = db.PingContext(ctx)
		assert.NoError(t, err)

		err = db.Close()
		require.NoError(t, err)
	})
}

func TestDB_Close(t *testing.T) {
	t.Run("SuccessfulClose", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.Close()
		assert.NoError(t, err)

		// Try to close again
		err = db.Close()
		assert.NoError(t, err)
	})

	t.Run("CloseEmptyDB", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.Close()
		assert.NoError(t, err)

		// Verify file still exists but is closed
		_, err = os.Stat(dbPath)
		assert.NoError(t, err)
	})
}

func TestDB_WALMode(t *testing.T) {
	t.Run("WALModeSet", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		// Query pragma to verify WAL mode is set
		var walMode string
		err = db.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&walMode)
		assert.NoError(t, err)
		assert.Equal(t, "wal", walMode)

		err = db.Close()
		require.NoError(t, err)
	})

	t.Run("WALModePersistence", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db1, err := New(context.Background(), dbPath)
		require.NoError(t, err)

		var walMode string
		err = db1.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&walMode)
		require.NoError(t, err)
		assert.Equal(t, "wal", walMode)

		err = db1.Close()
		require.NoError(t, err)

		db2, err := New(context.Background(), dbPath)
		require.NoError(t, err)

		err = db2.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&walMode)
		assert.NoError(t, err)
		assert.Equal(t, "wal", walMode)

		err = db2.Close()
		require.NoError(t, err)
	})

	t.Run("OtherPragmasSet", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		var foreignKeys int
		var busyTimeout int
		var synchronous int
		var cacheSize int
		var tempStore int

		err = db.QueryRowContext(t.Context(), "PRAGMA foreign_keys").Scan(&foreignKeys)
		assert.NoError(t, err)
		assert.Equal(t, 1, foreignKeys)

		err = db.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&busyTimeout)
		assert.NoError(t, err)
		assert.Equal(t, 5000, busyTimeout)

		err = db.QueryRowContext(t.Context(), "PRAGMA synchronous").Scan(&synchronous)
		assert.NoError(t, err)
		assert.Equal(t, 1, synchronous) // 1 = NORMAL

		err = db.QueryRowContext(t.Context(), "PRAGMA cache_size").Scan(&cacheSize)
		assert.NoError(t, err)
		assert.Equal(t, -64000, cacheSize)

		err = db.QueryRowContext(t.Context(), "PRAGMA temp_store").Scan(&tempStore)
		assert.NoError(t, err)
		assert.Equal(t, 2, tempStore) // 2 = MEMORY

		err = db.Close()
		require.NoError(t, err)
	})
}

func TestDB_RunInTx(t *testing.T) {
	t.Run("SuccessfulTransaction", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.ExecContext(t.Context(), "CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)")
			return err
		})
		require.NoError(t, err)

		// Verify table was created
		var tableName string
		err = db.QueryRowContext(t.Context(), "SELECT name FROM sqlite_master WHERE type='table' AND name='test'").Scan(&tableName)
		assert.NoError(t, err)
		assert.Equal(t, "test", tableName)

		err = db.Close()
		require.NoError(t, err)
	})

	t.Run("InsertAndQueryInTransaction", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.ExecContext(t.Context(), "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT, value INTEGER)")
			return err
		})
		require.NoError(t, err)

		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.ExecContext(t.Context(), "INSERT INTO test (name, value) VALUES (?, ?)", "test1", 100)
			return err
		})
		require.NoError(t, err)

		var count int
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM test").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)

		var name string
		var value int
		err = db.QueryRowContext(t.Context(), "SELECT name, value FROM test WHERE name='test1'").Scan(&name, &value)
		assert.NoError(t, err)
		assert.Equal(t, "test1", name)
		assert.Equal(t, 100, value)

		err = db.Close()
		require.NoError(t, err)
	})

	t.Run("TransactionWithMultipleStatements", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			// Create table
			_, err := tx.ExecContext(t.Context(), "CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)")
			if err != nil {
				return err
			}

			// Insert multiple rows
			for i := 0; i < 100; i++ {
				_, err := tx.ExecContext(t.Context(), "INSERT INTO test (value) VALUES (?)", fmt.Sprintf("value%d", i))
				if err != nil {
					return err
				}
			}

			// Query to verify
			var count int
			err = tx.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM test").Scan(&count)
			if err != nil {
				return err
			}

			return nil
		})
		require.NoError(t, err)

		var count int
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM test").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 100, count)

		err = db.Close()
		require.NoError(t, err)
	})

	t.Run("TransactionRollbackOnFailure", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.ExecContext(t.Context(), "CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)")
			return err
		})
		require.NoError(t, err)

		// Insert data
		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.ExecContext(t.Context(), "INSERT INTO test (value) VALUES (?)", "value1")
			return err
		})
		require.NoError(t, err)

		// Start a new transaction and insert more data, then fail
		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			_, _ = tx.ExecContext(t.Context(), "INSERT INTO test (value) VALUES (?)", "value2")
			return errTestSimulated
		})
		assert.Error(t, err)

		// Verify only the first value exists (transaction was rolled back)
		var count int
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM test").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)

		var name string
		err = db.QueryRowContext(t.Context(), "SELECT value FROM test WHERE value='value1'").Scan(&name)
		assert.NoError(t, err)
		assert.Equal(t, "value1", name)

		err = db.Close()
		require.NoError(t, err)
	})

	t.Run("TransactionCommitSuccess", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.ExecContext(t.Context(), "CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)")
			if err != nil {
				return err
			}

			_, err = tx.ExecContext(t.Context(), "INSERT INTO test (value) VALUES (?)", "committed_value")
			return err
		})
		require.NoError(t, err)

		var count int
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM test").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)

		var value string
		err = db.QueryRowContext(t.Context(), "SELECT value FROM test").Scan(&value)
		assert.NoError(t, err)
		assert.Equal(t, "committed_value", value)

		err = db.Close()
		require.NoError(t, err)
	})

	t.Run("TransactionWithContextCancellation", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.ExecContext(t.Context(), "CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)")
			return err
		})
		require.NoError(t, err)

		// Create a context that will be canceled
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err = db.RunInTx(ctx, func(tx *sql.Tx) error {
			// Insert data
			_, err := tx.ExecContext(t.Context(), "INSERT INTO test (value) VALUES (?)", "value1")
			if err != nil {
				return err
			}

			// Sleep to ensure context cancellation
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		// Should fail due to context cancellation
		assert.Error(t, err)

		err = db.Close()
		require.NoError(t, err)
	})
}

func TestDB_ConcurrentTransactions(t *testing.T) {
	t.Run("MultipleConcurrentTransactions", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		db, err := New(context.Background(), dbPath)
		require.NoError(t, err)
		require.NotNil(t, db)

		err = db.RunInTx(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.ExecContext(t.Context(), "CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)")
			return err
		})
		require.NoError(t, err)

		var wg sync.WaitGroup
		numTransactions := 100

		// Perform multiple concurrent transactions
		for i := 0; i < numTransactions; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				err := db.RunInTx(context.Background(), func(tx *sql.Tx) error {
					_, err := tx.ExecContext(t.Context(), "INSERT INTO test (value) VALUES (?)", fmt.Sprintf("value%d", n))
					return err
				})
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		var count int
		err = db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM test").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, numTransactions, count)

		err = db.Close()
		require.NoError(t, err)
	})
}

func isFile(path string) bool {
	info, _ := os.Stat(path)
	if info == nil {
		return false
	}
	_ = info
	return !info.IsDir()
}
