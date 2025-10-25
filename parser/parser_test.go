package parser

import (
	"testing"
)

func TestParserValid(t *testing.T) {
	data := "EXTRACT main.zip\nADD_TO_PATH main.rar"
	incs, err := Parse(data)
	if err != nil {
		t.Errorf("%v", err)
	}
	if incs[0].token != EXTRACT {
		t.Errorf("returning wrong token for first line")
	}
	if incs[1].token != ADD_TO_PATH {
		t.Errorf("returning wrong token for second line")
	}
}
