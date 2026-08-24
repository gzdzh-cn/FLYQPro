package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func AttachmentPeerDir(root, peerDeviceID string) string {
	if strings.TrimSpace(peerDeviceID) == "" {
		return filepath.Join(root, "_unclassified")
	}
	return filepath.Join(root, safeDirectoryName(peerDeviceID))
}

// IsPathWithin reports whether path is inside root without following a
// directory traversal outside the configured attachment root.
func IsPathWithin(path, root string) bool {
	cleanPath, pathErr := absoluteCleanPath(path)
	cleanRoot, rootErr := absoluteCleanPath(root)
	if pathErr != nil || rootErr != nil {
		return false
	}
	return cleanPath != cleanRoot && isWithin(cleanPath, cleanRoot)
}

func AttachmentTargetPath(root, peerDeviceID, fileName string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("附件保存目录不能为空")
	}
	name := safeFileName(fileName)
	if name == "" {
		name = "attachment"
	}
	dir := AttachmentPeerDir(root, peerDeviceID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	target := filepath.Join(dir, name)
	for index := 1; ; index++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return target, nil
		} else if err != nil {
			return "", err
		}
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		target = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, index, ext))
	}
}

func safeDirectoryName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_unclassified"
	}
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, value)
	return value
}

func MigrateAttachments(ctx context.Context, sourceRoot, targetRoot string, report func(AttachmentMigrationProgress)) (AttachmentMigrationResult, error) {
	var err error
	sourceRoot, err = absoluteCleanPath(sourceRoot)
	if err != nil {
		return AttachmentMigrationResult{}, err
	}
	targetRoot, err = absoluteCleanPath(targetRoot)
	if err != nil {
		return AttachmentMigrationResult{}, err
	}
	result := AttachmentMigrationResult{SourceRoot: sourceRoot, TargetRoot: targetRoot}
	if sourceRoot == targetRoot {
		result.Completed = true
		return result, nil
	}
	if sourceRoot == string(filepath.Separator) || targetRoot == string(filepath.Separator) || isWithin(targetRoot, sourceRoot) || isWithin(sourceRoot, targetRoot) {
		return result, fmt.Errorf("附件保存目录不能与旧目录相同或互相嵌套")
	}
	if info, statErr := os.Stat(targetRoot); statErr == nil && !info.IsDir() {
		return result, fmt.Errorf("附件保存路径不是目录")
	}
	if err := validateTargetRoot(targetRoot); err != nil {
		return result, err
	}
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return result, err
	}
	rows, err := ListAttachmentMigrationRows(ctx)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		path, pathErr := absoluteCleanPath(row.LocalPath)
		if pathErr == nil && isWithin(path, sourceRoot) && path != sourceRoot {
			result.Total++
		}
	}
	if report != nil {
		report(AttachmentMigrationProgress{Phase: "preparing", SourceRoot: sourceRoot, TargetRoot: targetRoot, Total: result.Total})
	}
	current := 0
	for _, row := range rows {
		sourcePath, sourceErr := absoluteCleanPath(row.LocalPath)
		if sourceErr != nil || !isWithin(sourcePath, sourceRoot) || sourcePath == sourceRoot {
			result.Skipped++
			continue
		}
		current++
		progress := func(phase, message string) {
			if report != nil {
				report(AttachmentMigrationProgress{Phase: phase, SourceRoot: sourceRoot, TargetRoot: targetRoot, Current: current, Total: result.Total, FileName: row.FileName, PeerDeviceID: row.PeerDeviceID, Migrated: result.Migrated, Skipped: result.Skipped, Failed: result.Failed, Unclassified: result.Unclassified, ErrorMessage: message})
			}
		}
		if _, err := os.Stat(sourcePath); err != nil {
			result.Skipped++
			progress("moving", "源文件不存在")
			continue
		}
		target, err := AttachmentTargetPath(targetRoot, row.PeerDeviceID, row.FileName)
		if err != nil {
			result.Failed++
			progress("moving", err.Error())
			continue
		}
		rollback, commit, err := prepareMoveAndVerify(sourcePath, target, row.FileSize, row.SHA256)
		if err != nil {
			result.Failed++
			progress("moving", err.Error())
			continue
		}
		if err := UpdateAttachmentLocalPath(ctx, row.AttachmentID, target); err != nil {
			_ = rollback()
			result.Failed++
			progress("moving", err.Error())
			continue
		}
		if err := commit(); err != nil {
			// The database still points at target, so retain target and surface
			// the error instead of pointing at a missing file.
			result.Failed++
			progress("moving", err.Error())
			continue
		}
		result.Migrated++
		if strings.TrimSpace(row.PeerDeviceID) == "" {
			result.Unclassified++
		}
		progress("moving", "")
	}
	if result.Failed > 0 {
		result.ErrorMessage = fmt.Sprintf("%d 个附件迁移失败", result.Failed)
		if report != nil {
			report(AttachmentMigrationProgress{Phase: "failed", SourceRoot: sourceRoot, TargetRoot: targetRoot, Current: current, Total: result.Total, Migrated: result.Migrated, Skipped: result.Skipped, Failed: result.Failed, Unclassified: result.Unclassified, ErrorMessage: result.ErrorMessage})
		}
		return result, fmt.Errorf("%s", result.ErrorMessage)
	}
	result.Completed = true
	if report != nil {
		report(AttachmentMigrationProgress{Phase: "completed", SourceRoot: sourceRoot, TargetRoot: targetRoot, Current: result.Total, Total: result.Total, Migrated: result.Migrated, Skipped: result.Skipped, Failed: result.Failed, Unclassified: result.Unclassified})
	}
	return result, nil
}

func prepareMoveAndVerify(source, target string, expectedSize int64, expectedSHA string) (rollback func() error, commit func() error, err error) {
	if err := os.Rename(source, target); err == nil {
		if verifyErr := verifyFile(target, expectedSize, expectedSHA); verifyErr == nil {
			return func() error { return os.Rename(target, source) }, func() error { return nil }, nil
		}
		_ = os.Rename(target, source)
	}
	input, err := os.Open(source)
	if err != nil {
		return nil, nil, err
	}
	defer input.Close()
	temporary := target + ".part"
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(temporary)
		return nil, nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return nil, nil, closeErr
	}
	if err := verifyFile(temporary, expectedSize, expectedSHA); err != nil {
		_ = os.Remove(temporary)
		return nil, nil, err
	}
	if err := os.Rename(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return nil, nil, err
	}
	return func() error { return os.Remove(target) }, func() error { return os.Remove(source) }, nil
}

func verifyFile(path string, expectedSize int64, expectedSHA string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if expectedSize > 0 && info.Size() != expectedSize {
		return fmt.Errorf("文件大小校验失败")
	}
	if expectedSHA == "" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedSHA) {
		return fmt.Errorf("文件哈希校验失败")
	}
	return nil
}

func isWithin(path, root string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	path, root = filepath.Clean(path), filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func absoluteCleanPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("附件保存目录不能为空")
	}
	path, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return filepath.Clean(path), nil
}

func validateTargetRoot(target string) error {
	parent := target
	for {
		info, err := os.Stat(parent)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("附件保存路径的父目录不是目录")
			}
			probe, err := os.CreateTemp(parent, ".flyqpro-write-test-*")
			if err != nil {
				return fmt.Errorf("附件保存路径不可写: %w", err)
			}
			name := probe.Name()
			_ = probe.Close()
			if err := os.Remove(name); err != nil {
				return fmt.Errorf("附件保存路径不可写: %w", err)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return fmt.Errorf("附件保存路径无效")
		}
		parent = next
	}
}
