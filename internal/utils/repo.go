package utils

import "strings"

// GetRepoIdentifier 返回根据仓库URL和分支生成的作者_仓库名标识符
func GetRepoIdentifier(url string, branch string) string {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")

	repoName := url[strings.LastIndex(url, "/")+1:]
	
	author := ""
	lastSlash := strings.LastIndex(url, "/")
	if lastSlash != -1 {
		prefix := url[:lastSlash]
		if strings.Contains(prefix, ":") {
			parts := strings.Split(prefix, ":")
			prefix = parts[len(parts)-1]
		}
		lastSlashPrefix := strings.LastIndex(prefix, "/")
		if lastSlashPrefix != -1 {
			author = prefix[lastSlashPrefix+1:]
		} else {
			author = prefix
		}
	}

	if dotIdx := strings.LastIndex(author, "."); dotIdx != -1 {
		author = author[dotIdx+1:]
	}

	identifier := ""
	if author != "" {
		identifier = author + "_" + repoName
	} else {
		identifier = repoName
	}

	if branch != "" && branch != "master" && branch != "main" {
		identifier = identifier + "_" + branch
	}
	
	// Replace any invalid characters for tags or paths
	identifier = strings.ReplaceAll(identifier, "/", "_")
	identifier = strings.ReplaceAll(identifier, ".", "_")
	return identifier
}
