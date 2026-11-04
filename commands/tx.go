package commands

import (
	"database/sql"

	"charm.land/log/v2"
)

// FinishTx commits tx when workErr is nil and rolls it back otherwise.
// A rollback failure is logged. workErr is the error returned.
func FinishTx(tx *sql.Tx, workErr error) (err error) {
	failed := workErr != nil
	if failed {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Error("rollback", "err", rollbackErr)
		}
		err = workErr
		return
	}
	err = tx.Commit()
	if err != nil {
		log.Error("commit", "err", err)
	}
	return
}
