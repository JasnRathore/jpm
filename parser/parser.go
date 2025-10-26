package parser

import (
	"errors"
	"fmt"
	"jpm/lib"
	"jpm/model"
	"path/filepath"
	"strings"
)

type Token int

const (
	DOWNLOAD Token = iota
	EXTRACT
	EXTRACT_TAR
	EXTRACT_TAR_GZ
	MOVE
	COPY
	RENAME
	DELETE
	CHMOD
	ADD_TO_PATH
	SET_LOCATION
	RUN_SCRIPT
	INVALID
)

var tokenMap = map[string]Token{
	"DOWNLOAD":      DOWNLOAD,
	"EXTRACT":       EXTRACT,
	"EXTRACT_TAR":   EXTRACT_TAR,
	"EXTRACT_TARGZ": EXTRACT_TAR_GZ,
	"MOVE":          MOVE,
	"COPY":          COPY,
	"RENAME":        RENAME,
	"DELETE":        DELETE,
	"CHMOD":         CHMOD,
	"ADD_TO_PATH":   ADD_TO_PATH,
	"SET_LOCATION":  SET_LOCATION,
	"RUN_SCRIPT":    RUN_SCRIPT,
}

func stringToToken(tokenStr string) Token {
	if token, exists := tokenMap[strings.ToUpper(tokenStr)]; exists {
		return token
	}
	return INVALID
}

// Instruction represents a parsed instruction with validation
type Instruction struct {
	Token      Token
	Args       []string
	RawLine    string
	LineNumber int
}

// Validate checks if the instruction has valid arguments
func (inc *Instruction) Validate() error {
	switch inc.Token {
	case EXTRACT, EXTRACT_TAR, EXTRACT_TAR_GZ:
		if len(inc.Args) < 1 || len(inc.Args) > 2 {
			return fmt.Errorf("line %d: EXTRACT requires 1-2 arguments (source [destination])", inc.LineNumber)
		}
	case ADD_TO_PATH, SET_LOCATION, DELETE, CHMOD:
		if len(inc.Args) != 1 {
			return fmt.Errorf("line %d: %v requires exactly 1 argument", inc.LineNumber, inc.Token)
		}
	case MOVE, COPY, RENAME:
		if len(inc.Args) != 2 {
			return fmt.Errorf("line %d: %v requires exactly 2 arguments (source destination)", inc.LineNumber, inc.Token)
		}
	case RUN_SCRIPT:
		if len(inc.Args) < 1 {
			return fmt.Errorf("line %d: RUN_SCRIPT requires at least 1 argument", inc.LineNumber)
		}
	}
	return nil
}

// Run executes the instruction and updates the installation model
func (inc *Instruction) Run(ins *model.Installed, workDir string) error {
	switch inc.Token {
	case EXTRACT:
		return inc.runExtract(workDir)
	case EXTRACT_TAR:
		return inc.runExtractTar(workDir, false)
	case EXTRACT_TAR_GZ:
		return inc.runExtractTar(workDir, true)
	case ADD_TO_PATH:
		return inc.runAddToPath(ins, workDir)
	case SET_LOCATION:
		return inc.runSetLocation(ins, workDir)
	case DELETE:
		return inc.runDelete(workDir)
	case MOVE:
		return inc.runMove(workDir)
	case COPY:
		return inc.runCopy(workDir)
	case RENAME:
		return inc.runRename(workDir)
	case CHMOD:
		return inc.runChmod(workDir)
	default:
		return fmt.Errorf("unimplemented instruction: %v", inc.Token)
	}
}

func (inc *Instruction) runExtract(workDir string) error {
	source := filepath.Join(workDir, inc.Args[0])
	dest := workDir
	if len(inc.Args) > 1 {
		dest = filepath.Join(workDir, inc.Args[1])
	}

	_, err := lib.ExtractZip(source, dest)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", source, err)
	}

	// Optionally delete the archive after extraction
	if err := lib.Delete(source); err != nil {
		fmt.Printf("Warning: failed to delete archive %s: %v\n", source, err)
	}
	return nil
}

func (inc *Instruction) runExtractTar(workDir string, gzipped bool) error {
	source := filepath.Join(workDir, inc.Args[0])
	dest := workDir
	if len(inc.Args) > 1 {
		dest = filepath.Join(workDir, inc.Args[1])
	}

	// You'll need to implement ExtractTar in lib/lib.go
	var err error
	if gzipped {
		_, err = lib.ExtractTarGz(source, dest)
	} else {
		_, err = lib.ExtractTar(source, dest)
	}

	if err != nil {
		return fmt.Errorf("failed to extract tar %s: %w", source, err)
	}

	if err := lib.Delete(source); err != nil {
		fmt.Printf("Warning: failed to delete archive %s: %v\n", source, err)
	}

	return nil
}

func (inc *Instruction) runAddToPath(ins *model.Installed, workDir string) error {
	pathToAdd := inc.Args[0]
	if !filepath.IsAbs(pathToAdd) {
		pathToAdd = filepath.Join(workDir, pathToAdd)
	}

	sysPath, err := lib.AddToPath(pathToAdd)
	if err != nil {
		return fmt.Errorf("failed to add to PATH: %w", err)
	}

	fmt.Printf("Added to PATH: %s\n", pathToAdd)
	ins.SysPath = sysPath
	return nil
}

func (inc *Instruction) runSetLocation(ins *model.Installed, workDir string) error {
	location := filepath.Join(workDir, inc.Args[0])
	ins.Location = location
	return nil
}

func (inc *Instruction) runDelete(workDir string) error {
	target := filepath.Join(workDir, inc.Args[0])
	return lib.Delete(target)
}

func (inc *Instruction) runMove(workDir string) error {
	src := filepath.Join(workDir, inc.Args[0])
	dst := filepath.Join(workDir, inc.Args[1])
	return lib.Move(src, dst)
}

func (inc *Instruction) runCopy(workDir string) error {
	src := filepath.Join(workDir, inc.Args[0])
	dst := filepath.Join(workDir, inc.Args[1])
	return lib.Copy(src, dst)
}

func (inc *Instruction) runRename(workDir string) error {
	return inc.runMove(workDir)
}

func (inc *Instruction) runChmod(workDir string) error {
	target := filepath.Join(workDir, inc.Args[0])
	return lib.MakeExecutable(target)
}

// Parser holds parsing state and configuration
type Parser struct {
	AllowComments bool
	StrictMode    bool
}

// NewParser creates a new parser with default settings
func NewParser() *Parser {
	return &Parser{
		AllowComments: true,
		StrictMode:    true,
	}
}

// Parse parses instruction text into a list of validated instructions
func (p *Parser) Parse(data string) ([]Instruction, error) {
	if strings.TrimSpace(data) == "" {
		return nil, errors.New("empty instruction set")
	}

	lines := strings.Split(data, "\n")
	var instructions []Instruction

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || (p.AllowComments && strings.HasPrefix(line, "#")) {
			continue
		}

		instruction, err := p.parseLine(line, lineNum+1)
		if err != nil {
			return nil, err
		}

		if err := instruction.Validate(); err != nil {
			return nil, err
		}

		instructions = append(instructions, instruction)
	}

	if len(instructions) == 0 {
		return nil, errors.New("no valid instructions found")
	}

	return instructions, nil
}

// parseLine parses a single instruction line
func (p *Parser) parseLine(line string, lineNum int) (Instruction, error) {
	// Handle quoted arguments (e.g., paths with spaces)
	parts := p.smartSplit(line)

	if len(parts) < 1 {
		return Instruction{}, fmt.Errorf("line %d: empty instruction", lineNum)
	}

	token := stringToToken(parts[0])
	if token == INVALID {
		return Instruction{}, fmt.Errorf("line %d: invalid command '%s'", lineNum, parts[0])
	}

	return Instruction{
		Token:      token,
		Args:       parts[1:],
		RawLine:    line,
		LineNumber: lineNum,
	}, nil
}

// smartSplit splits a line while respecting quoted strings
func (p *Parser) smartSplit(line string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)

	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		switch {
		case (ch == '"' || ch == '\'') && !inQuotes && (i == 0 || runes[i-1] == ' '):
			// Start of quoted string (only at word boundaries)
			inQuotes = true
			quoteChar = ch
		case ch == quoteChar && inQuotes:
			// End of quoted string
			inQuotes = false
			quoteChar = 0
		case ch == ' ' && !inQuotes:
			// Word separator outside quotes
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			// Regular character (including backslashes)
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	// If we ended with unclosed quotes, return error indication
	if inQuotes && len(parts) == 0 {
		return []string{line}
	}

	return parts
}

// Convenience function for backward compatibility
func Parse(data string) ([]Instruction, error) {
	parser := NewParser()
	return parser.Parse(data)
}
