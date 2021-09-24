package operationplanner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getCommonRoot(t *testing.T) {
	cases := []struct {
		Current string
		Update  string
		Result  string
	}{
		{Current: "", Update: "/users/alma/xxx", Result: "/users/alma/"},
		{Current: "", Update: "/users/alma/xxx/", Result: "/users/alma/xxx/"},
		{Current: "/users/alma/xxx/", Update: "/users/alma/xxx/", Result: "/users/alma/xxx/"},
		{Current: "/users/alma/xxx/", Update: "/users/alma/xxx", Result: "/users/alma/xxx/"},
		{Current: "/users/alma/xxx/", Update: "/users/alma/some_file", Result: "/users/alma/"},
		{Current: "/users/alma/xxx/", Update: "/users/alma/yyy/", Result: "/users/alma/"},
		{Current: "/users/alma/xxx/", Update: "/", Result: "/"},
		{Current: "/users/", Update: "/nope", Result: "/"},
		{Current: "/users/", Update: "/users", Result: "/users/"},
		{Current: "/users/", Update: "/users/a", Result: "/users/"},
		{Current: "/", Update: "/", Result: "/"},
		{Current: "/", Update: "/kortefa/", Result: "/"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("Update %s with %s should be %s", c.Current, c.Update, c.Result), func(t *testing.T) {
			plan := OperationPlan{CommonRoot: c.Current}
			updated := getCommonRoot(&plan, c.Update)

			assert.Equal(t, c.Result, updated)
		})
	}
}
