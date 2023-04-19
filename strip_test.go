package muxter

import "testing"

func TestStripPathDepth(t *testing.T) {
	testcases := []struct {
		Input  string
		Depth  int
		Output string
	}{
		{
			Input:  "/input",
			Depth:  0,
			Output: "/input",
		},
		{
			Input:  "/my/path",
			Depth:  20,
			Output: "/",
		},
		{
			Input:  "/some/long/segment",
			Depth:  2,
			Output: "/segment",
		},
		{
			Input:  "some/long/segment",
			Depth:  2,
			Output: "/segment",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Input, func(t *testing.T) {
			if result := stripDepth(tc.Input, tc.Depth); result != tc.Output {
				t.Errorf("expected %q but got %q", tc.Output, result)
			}
		})
	}
}

func TestFail(t *testing.T) {
	t.Error("Test!")
}
