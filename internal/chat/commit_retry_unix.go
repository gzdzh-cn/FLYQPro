//go:build !windows

package chat

func renameV3WithRetry(persistence transferPersistenceIO, source, target string) error {
	return persistence.Rename(source, target)
}
