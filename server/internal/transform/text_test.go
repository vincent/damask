package transform

import (
	"testing"
)

func Test_generateImageOfText(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		textContent string
		fgColorHex  string
		bgColorHex  string
		ttfFontName string
		fontSize    float64
		want        []byte
		wantErr     bool
	}{
		// TODO: Add test cases.
		{
			textContent: `Lorem ipsum dolor sit amet, consectetur adipiscing elit.
Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.

Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.
Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.`,
			fgColorHex:  "#000000",
			bgColorHex:  "#ffffff",
			ttfFontName: "goregular",
			fontSize:    0,
			want:        nil, // TODO: set expected output
			wantErr:     false,
		},
		{
			textContent: `Lorem ipsum dolor`,
			fgColorHex:  "#000000",
			bgColorHex:  "#ffffff",
			ttfFontName: "goregular",
			fontSize:    0,
			want:        nil, // TODO: set expected output
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotErr := GenerateImageOfText(t.Context(), tt.textContent, tt.fgColorHex, tt.bgColorHex, tt.ttfFontName, tt.fontSize)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("generateImageOfText() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("generateImageOfText() succeeded unexpectedly")
			}
		})
	}
}
