package ion

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

type testingFunc func(t *testing.T, path string)

const goodPath = "ion-tests/iontestdata/good"
const badPath = "ion-tests/iontestdata/bad"

func TestRoundTripBinToTxt(t *testing.T) {
	readFilesAndTest(t, goodPath, func(t *testing.T, path string) {
		testRoundTripBinToTxt(t, path)
	})
}

func TestRoundTripTxtToBin(t *testing.T) {
	readFilesAndTest(t, goodPath, func(t *testing.T, path string) {
		testRoundTripTxtToBin(t, path)
	})
}

func TestLoadBad(t *testing.T) {
	readFilesAndTest(t, badPath, func(t *testing.T, path string) {
		testLoadBad(t, path)
	})
}

func readFilesAndTest(t *testing.T, path string, tf testingFunc) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		fp := filepath.Join(path, file.Name())
		if file.IsDir() {
			readFilesAndTest(t, fp, tf)
		} else if skipNonIonFiles(file.Name()) {
			continue
		} else {
			t.Run(fp, func(t *testing.T) {
				tf(t, fp)
			})
		}
	}
}

func skipNonIonFiles(fn string) bool {
	ion, _ := regexp.MatchString(`.ion$`, fn)
	bin, _ := regexp.MatchString(`.10n$`, fn)

	return !ion && !bin
}

func testRoundTripBinToTxt(t *testing.T, fp string) {
	bytes := loadFile(t, fp)

	buf, err := writeToBinary(bytes)
	str, err := writeToText(string(buf))

	roundTripAssertion(t, fp, buf, str, err)
}

func testRoundTripTxtToBin(t *testing.T, fp string) {
	bytes := loadFile(t, fp)

	str, err := writeToText(string(bytes))
	buf, err := writeToBinary([]byte(str))

	roundTripAssertion(t, fp, buf, str, err)
}

func loadFile(t *testing.T, path string) []byte {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeToText(in string) (string, error) {
	r := NewReaderStr(in)
	str := strings.Builder{} // empty string builder
	w := NewTextWriter(&str) // text writer with string builder

	err := writeValue(r, w)
	return str.String(), err
}

func writeToBinary(in []byte) ([]byte, error) {
	r := NewReaderBytes(in)
	buf := bytes.Buffer{}      // empty buffer for binary writer
	w := NewBinaryWriter(&buf) // binary writer with buffer

	err := writeValue(r, w)
	return buf.Bytes(), err
}

func writeValue(r Reader, w Writer) error {
	d := NewDecoder(r)
	for {
		v, err := d.Decode() // parse all the values in the reader, into v
		if err == ErrNoInput {
			break
		}
		if err != nil {
			return err
		}

		err = MarshalTo(w, v) // write parsed value, v, into the writer
		if err != nil {
			return err
		}
	}

	err := w.Finish()
	if err != nil {
		return err
	}

	return nil
}

func roundTripAssertion(t *testing.T, path string, buf []byte, str string, err error) {
	var txtValue interface{}
	var binValue interface{}

	errT := UnmarshalStr(str, &txtValue) // put contents of text writer into item
	errB := Unmarshal(buf, &binValue)    // put contents of binary writer into itemB

	if err != nil {
		t.Error(err)
	} else if errB != nil || errT != nil {
		t.Errorf("Failed on unmarshaling while testing: " + path)
	} else if !reflect.DeepEqual(txtValue, binValue) {
		t.Errorf("Round trip test failed on: " + path)
	}
}

func testLoadBad(t *testing.T, fp string) {
	file, err := os.Open(fp)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	r := NewReader(file)

	for r.Next() {
		if !r.IsNull() {
			fmt.Println(r.Type())
		}
	}

	if r.Err() == nil {
		t.Fatal("Should have failed loading \"" + fp + "\".")
	} else {
		fmt.Println("expectedly failed loading " + r.Err().Error())
	}
}
