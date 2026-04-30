package sessionconfig

import "path/filepath"

var discobotSystemRoots = []string{
	"/opt/discobot",
	"/usr/local/share/discobot",
	"/usr/share/discobot",
}

func discobotSystemPaths(rel string) []string {
	paths := make([]string, 0, len(discobotSystemRoots))
	for _, root := range discobotSystemRoots {
		paths = append(paths, filepath.Join(root, rel))
	}
	return paths
}
