package parser

import (
	"errors"
	"fmt"
	"jpm/lib"
	"path"
	"strings"
)

type Token int

const (
	EXTRACT Token = iota
	ADD_TO_PATH
	INVALID
)

func stringToToken(tokenStr string) Token {
	switch strings.ToUpper(tokenStr) {
	case "EXTRACT":
		return EXTRACT
	case "ADD_TO_PATH":
		return ADD_TO_PATH
	default:
		return INVALID
	}
}

/*
EXTRACT main.zip
ADD_TO_PATH /main
*/
type Instruction struct {
	token Token
	data  string
}

func (inc *Instruction) Run() {
	switch inc.token {
	case EXTRACT:
		fullPath := path.Join("bin", inc.data)
		lib.ExtractZip(fullPath, "bin")
		lib.Delete(fullPath)
	case ADD_TO_PATH:
		err := lib.AddToPath(inc.data)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Added To Path: %s", inc.data)
		return
	}
}

func newInstruction(token Token, data string) Instruction {
	return Instruction{token: token, data: data}
}

func Parse(data string) ([]Instruction, error) {
	instructions := strings.Split(data, "\n")
	var TokenizedInstructions []Instruction
	for index, inc := range instructions {
		parts := strings.Split(strings.Trim(inc, " "), " ")
		if len(parts) > 2 {
			return nil, errors.New(fmt.Sprintf("Too Many Words in one Instruction at line %d", index+1))
		}
		if len(parts) < 2 {
			return nil, errors.New(fmt.Sprintf("Too few Words in one Instruction at line %d", index+1))
		}
		incToken := stringToToken(parts[0])
		if incToken == INVALID {
			return nil, errors.New(fmt.Sprintf("Invalid Command at line %d", index+1))
		}
		TokenizedInstructions = append(TokenizedInstructions, newInstruction(incToken, parts[1]))
	}
	return TokenizedInstructions, nil
}
