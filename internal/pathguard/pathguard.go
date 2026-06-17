package pathguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ResolveRead(requested string, roots []string) (string, error) {
	if requested == "" {
		return "", errors.New("path is required")
	}
	resolved, err := filepath.Abs(filepath.Clean(requested))
	if err != nil {
		return "", err
	}
	resolved, err = filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", err
	}
	if info.Mode()&(os.ModeDevice|os.ModeNamedPipe|os.ModeSocket) != 0 {
		return "", fmt.Errorf("special file is not allowed: %s", requested)
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		rr, err := filepath.Abs(filepath.Clean(root))
		if err != nil {
			continue
		}
		rr, err = filepath.EvalSymlinks(rr)
		if err != nil {
			continue
		}
		if inside(resolved, rr) {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("path denied: %s", requested)
}

func inside(path, root string) bool {
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}
