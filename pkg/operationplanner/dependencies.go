package operationplanner

import (
	"fmt"
	"github.com/thoas/go-funk"
	"io/ioutil"
	"path/filepath"
	"regexp"
)

func getDependencies(path string) ([]string, error) {
	body, err := ioutil.ReadFile(path)
	if err != nil {
		return []string{}, fmt.Errorf("reading HCL file %s: %w", path, err)
	}

	bodystr := string(body)
	res := []string{}

	regex := regexp.MustCompile(`dependency\s+[\w\-_"]+\s+{\s+.*?config_path\s+=\s+"([./\w_\-]+)"`)
	matches := regex.FindAllStringSubmatch(bodystr, -1)
	if matches != nil && len(matches) > 0 {
		deps := funk.Map(matches, func(m []string) string {
			return filepath.Join(filepath.Dir(path), m[1])
		}).([]string)
		res = append(res, deps...)
	}

	regex = regexp.MustCompile(`terraform\s+{\s+.*?source\s+=\s+"([./\w_\-]+)"`)
	matches = regex.FindAllStringSubmatch(bodystr, -1)
	if matches != nil && len(matches) > 0 {
		deps := funk.Map(matches, func(m []string) string {
			return filepath.Join(filepath.Dir(path), m[1])
		}).([]string)
		res = append(res, deps...)
	}

	regex = regexp.MustCompile(`module\s+[\w\-_"]+\s+{\s+.*?source\s+=\s+"([./\w_\-]+)"`)
	matches = regex.FindAllStringSubmatch(bodystr, -1)
	if matches != nil && len(matches) > 0 {
		deps := funk.Map(matches, func(m []string) string {
			return filepath.Join(filepath.Dir(path), m[1])
		}).([]string)
		res = append(res, deps...)
	}

	return res, nil
}
