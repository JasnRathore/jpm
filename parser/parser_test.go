package parser

import (
	"testing"
)

func TestParserBasicInstructions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantLen     int
		wantTokens  []Token
		shouldError bool
	}{
		{
			name:       "Simple extract and add to path",
			input:      "EXTRACT main.zip\nADD_TO_PATH main",
			wantLen:    2,
			wantTokens: []Token{EXTRACT, ADD_TO_PATH},
		},
		{
			name:       "Extract with destination",
			input:      "EXTRACT main.zip extracted/",
			wantLen:    1,
			wantTokens: []Token{EXTRACT},
		},
		{
			name:       "Multiple extracts",
			input:      "EXTRACT app.zip\nEXTRACT libs.zip libs/\nEXTRACT data.zip data/",
			wantLen:    3,
			wantTokens: []Token{EXTRACT, EXTRACT, EXTRACT},
		},
		{
			name:        "Empty input",
			input:       "",
			shouldError: true,
		},
		{
			name:        "Invalid command",
			input:       "INVALID_CMD arg",
			shouldError: true,
		},
		{
			name:       "Comments and empty lines",
			input:      "# This is a comment\nEXTRACT app.zip\n\n# Another comment\nADD_TO_PATH bin",
			wantLen:    2,
			wantTokens: []Token{EXTRACT, ADD_TO_PATH},
		},
		{
			name:       "Tar archives",
			input:      "EXTRACT_TAR app.tar\nEXTRACT_TARGZ lib.tar.gz libs/",
			wantLen:    2,
			wantTokens: []Token{EXTRACT_TAR, EXTRACT_TAR_GZ},
		},
		{
			name:       "File operations",
			input:      "MOVE temp/app bin/app\nCOPY config.txt backup/config.txt\nDELETE temp/",
			wantLen:    3,
			wantTokens: []Token{MOVE, COPY, DELETE},
		},
		{
			name:       "Make executable",
			input:      "EXTRACT app.zip\nCHMOD app/binary",
			wantLen:    2,
			wantTokens: []Token{EXTRACT, CHMOD},
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instructions, err := parser.Parse(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(instructions) != tt.wantLen {
				t.Errorf("got %d instructions, want %d", len(instructions), tt.wantLen)
				return
			}

			for i, want := range tt.wantTokens {
				if instructions[i].Token != want {
					t.Errorf("instruction %d: got token %v, want %v", i, instructions[i].Token, want)
				}
			}
		})
	}
}

func TestParserQuotedArguments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantArgs [][]string
	}{
		{
			name:     "Quoted path with spaces",
			input:    `MOVE "Program Files/app" bin/app`,
			wantArgs: [][]string{{"Program Files/app", "bin/app"}},
		},
		{
			name:     "Single quotes",
			input:    `COPY 'my file.txt' 'backup/my file.txt'`,
			wantArgs: [][]string{{"my file.txt", "backup/my file.txt"}},
		},
		{
			name:     "Mixed quotes",
			input:    `EXTRACT "my archive.zip" 'output folder'`,
			wantArgs: [][]string{{"my archive.zip", "output folder"}},
		},
		{
			name:     "Windows paths without spaces",
			input:    `MOVE app\bin\program.exe dest\program.exe`,
			wantArgs: [][]string{{`app\bin\program.exe`, `dest\program.exe`}},
		},
		{
			name:     "Windows paths with spaces (quoted)",
			input:    `MOVE "C:\Program Files\app.exe" "D:\My Apps\app.exe"`,
			wantArgs: [][]string{{`C:\Program Files\app.exe`, `D:\My Apps\app.exe`}},
		},
		{
			name:     "Mixed forward and backward slashes",
			input:    `COPY app/bin\file.txt backup\app/file.txt`,
			wantArgs: [][]string{{`app/bin\file.txt`, `backup\app/file.txt`}},
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instructions, err := parser.Parse(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			for i, want := range tt.wantArgs {
				got := instructions[i].Args
				if len(got) != len(want) {
					t.Errorf("instruction %d: got %d args, want %d", i, len(got), len(want))
					continue
				}

				for j := range want {
					if got[j] != want[j] {
						t.Errorf("instruction %d arg %d: got %q, want %q", i, j, got[j], want[j])
					}
				}
			}
		})
	}
}

func TestParserValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "EXTRACT with no args",
			input:       "EXTRACT",
			shouldError: true,
		},
		{
			name:        "EXTRACT with too many args",
			input:       "EXTRACT a b c",
			shouldError: true,
		},
		{
			name:        "ADD_TO_PATH with no args",
			input:       "ADD_TO_PATH",
			shouldError: true,
		},
		{
			name:        "ADD_TO_PATH with too many args",
			input:       "ADD_TO_PATH a b",
			shouldError: true,
		},
		{
			name:        "MOVE with one arg",
			input:       "MOVE source",
			shouldError: true,
		},
		{
			name:        "COPY with no args",
			input:       "COPY",
			shouldError: true,
		},
		{
			name:        "Valid MOVE",
			input:       "MOVE src dst",
			shouldError: false,
		},
		{
			name:        "Valid EXTRACT with dest",
			input:       "EXTRACT app.zip extracted/",
			shouldError: false,
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.input)

			if tt.shouldError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParserComplexScenarios(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "Complete installation flow",
			input: `# Download and extract main application
EXTRACT app-v1.2.3.zip
CHMOD app/bin/myapp

# Extract additional libraries
EXTRACT_TARGZ libs.tar.gz libs/

# Setup configuration
COPY config/default.conf app/config.conf
MOVE app/ /opt/myapp

# Add to system PATH
ADD_TO_PATH /opt/myapp/bin

# Cleanup
DELETE libs.tar.gz`,
		},
		{
			name: "Multi-component application",
			input: `EXTRACT frontend.zip
EXTRACT backend.zip
EXTRACT_TAR database.tar
MOVE frontend/ app/frontend
MOVE backend/ app/backend
MOVE database/ app/db
ADD_TO_PATH app/backend/bin
CHMOD app/backend/bin/server
SET_LOCATION app/`,
		},
		{
			name: "Windows-style paths",
			input: `EXTRACT app.zip
MOVE app\bin\program.exe "C:\Program Files\MyApp\program.exe"
ADD_TO_PATH "C:\Program Files\MyApp"`,
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instructions, err := parser.Parse(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(instructions) == 0 {
				t.Errorf("expected instructions but got none")
			}

			// Verify all instructions are valid
			for i, inst := range instructions {
				if err := inst.Validate(); err != nil {
					t.Errorf("instruction %d validation failed: %v", i, err)
				}
			}
		})
	}
}

func TestParserEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
	}{
		{
			name:        "Only comments",
			input:       "# Comment 1\n# Comment 2\n# Comment 3",
			shouldError: true, // No actual instructions
		},
		{
			name:        "Only whitespace",
			input:       "   \n\t\n  \n",
			shouldError: true,
		},
		{
			name:        "Trailing whitespace",
			input:       "EXTRACT app.zip   \nADD_TO_PATH bin   ",
			shouldError: false,
		},
		{
			name:        "Mixed case commands",
			input:       "extract app.zip\nadd_to_path bin",
			shouldError: false,
		},
		{
			name:        "Unicode in filenames",
			input:       "EXTRACT файл.zip\nMOVE 文件.txt backup/",
			shouldError: false,
		},
		{
			name:        "Command in middle of line",
			input:       "some text EXTRACT app.zip",
			shouldError: true, // Invalid format
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.input)

			if tt.shouldError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestInstructionRun(t *testing.T) {
	// Note: These would need actual file system setup
	// This is a structure for integration tests
	t.Skip("Integration tests require file system setup")

	// Example structure:
	// - Create temp directory
	// - Create test files
	// - Run instructions
	// - Verify results
	// - Cleanup
}

func BenchmarkParser(b *testing.B) {
	input := `# Installation instructions
EXTRACT app.zip
EXTRACT_TARGZ libs.tar.gz libs/
MOVE app/ /opt/myapp
COPY config.conf /opt/myapp/config.conf
CHMOD /opt/myapp/bin/app
ADD_TO_PATH /opt/myapp/bin
DELETE temp/`

	parser := NewParser()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = parser.Parse(input)
	}
}
