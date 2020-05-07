package ion

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

type testingFunc func(t *testing.T, path string)

const goodPath = "ion-tests/iontestdata/good"

var binaryRoundTripSkipList = []string{
	"allNulls.ion",
	"bigInts.ion",
	"clobWithNonAsciiCharacter.10n",
	"clobs.ion",
	"decimal64BitBoundary.ion",
	"decimals.ion",
	"floats.ion",
	"intBigSize1201.10n",
	"intBigSize13.10n",
	"intBigSize14.10n",
	"intBigSize16.10n",
	"intBigSize256.10n",
	"intBigSize256.ion",
	"intBigSize512.ion",
	"intLongMaxValuePlusOne.10n",
	"localSymbolTableImportZeroMaxId.ion",
	"nullDecimal.10n",
	"nulls.ion",
	"structWhitespace.ion",
	"subfieldInt.ion",
	"subfieldUInt.ion",
	"subfieldVarInt.ion",
	"subfieldVarUInt.ion",
	"subfieldVarUInt15bit.ion",
	"subfieldVarUInt16bit.ion",
	"subfieldVarUInt32bit.ion",
	"symbolEmpty.ion",
	"symbols.ion",
	"testfile22.ion",
	"testfile23.ion",
	"testfile31.ion",
	"testfile35.ion",
	"testfile37.ion",
	"utf16.ion",
	"utf32.ion",
	"whitespace.ion",
}

var textRoundTripSkipList = []string{
	"allNulls.ion",
	"annotations.ion",
	"bigInts.ion",
	"clobWithNonAsciiCharacter.10n",
	"clobs.ion",
	"decimal64BitBoundary.ion",
	"decimal_values.ion",
	"decimals.ion",
	"decimalsWithUnderscores.ion",
	"float_zeros.ion",
	"floats.ion",
	"floatsVsDecimals.ion",
	"intBigSize1201.10n",
	"intBigSize13.10n",
	"intBigSize14.10n",
	"intBigSize16.10n",
	"intBigSize256.10n",
	"intBigSize256.ion",
	"intBigSize512.ion",
	"intLongMaxValuePlusOne.10n",
	"localSymbolTableImportZeroMaxId.ion",
	"notVersionMarkers.ion",
	"nullDecimal.10n",
	"nulls.ion",
	"structWhitespace.ion",
	"subfieldInt.ion",
	"subfieldUInt.ion",
	"subfieldVarInt.ion",
	"subfieldVarUInt.ion",
	"subfieldVarUInt15bit.ion",
	"subfieldVarUInt16bit.ion",
	"subfieldVarUInt32bit.ion",
	"symbolEmpty.ion",
	"symbols.ion",
	"symbols.ion",
	"symbols.ion",
	"systemSymbols.ion",
	"systemSymbolsAsAnnotations.ion",
	"testfile22.ion",
	"testfile23.ion",
	"testfile24.ion",
	"testfile31.ion",
	"testfile35.ion",
	"testfile37.ion",
	"utf16.ion",
	"utf32.ion",
	"whitespace.ion",
	"zeroFloats.ion",
}

func TestBinaryRoundTrip(t *testing.T) {
	readFilesAndTest(t, goodPath, binaryRoundTripSkipList, func(t *testing.T, path string) {
		binaryRoundTrip(t, path)
	})
}

func TestTextRoundTrip(t *testing.T) {
	readFilesAndTest(t, goodPath, textRoundTripSkipList, func(t *testing.T, path string) {
		textRoundTrip(t, path)
	})
}

func binaryRoundTrip(t *testing.T, fp string) {
	b := loadFile(t, fp)

	// Make a binary writer from the file
	br := NewReaderBytes(b)
	buf := bytes.Buffer{}
	bw := NewBinaryWriter(&buf)
	writeToWriterFromReader(t, br, bw)
	bw.Finish()

	// Make a text writer from the binary writer
	r := NewReaderBytes(buf.Bytes())
	str := strings.Builder{}
	w := NewTextWriter(&str)
	writeToWriterFromReader(t, r, w)
	w.Finish()

	// Make another binary writer using the text writer
	br2 := NewReaderStr(str.String())
	buf2 := bytes.Buffer{}
	bw2 := NewBinaryWriter(&buf2)
	writeToWriterFromReader(t, br2, bw2)
	bw2.Finish()

	// Compare the 2 binary writers
	if !reflect.DeepEqual(bw, bw2) {
		t.Errorf("Round trip test failed on: " + fp)
	}
}

func textRoundTrip(t *testing.T, fp string) {
	b := loadFile(t, fp)

	// Make a binary writer from the file
	r := NewReaderBytes(b)
	str := strings.Builder{}
	w := NewTextWriter(&str)
	writeToWriterFromReader(t, r, w)
	w.Finish()

	// Make a text writer from the binary writer
	br := NewReaderStr(str.String())
	buf := bytes.Buffer{}
	bw := NewBinaryWriter(&buf)
	writeToWriterFromReader(t, br, bw)
	bw.Finish()

	// Make another binary writer using the text writer
	r2 := NewReaderBytes(buf.Bytes())
	str2 := strings.Builder{}
	w2 := NewTextWriter(&str2)
	writeToWriterFromReader(t, r2, w2)
	w2.Finish()

	//compare the 2 text writers
	if !reflect.DeepEqual(w, w2) {
		t.Errorf("Round trip test failed on: " + fp)
	}
}

func readFilesAndTest(t *testing.T, path string, skipList []string, tf testingFunc) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		fp := filepath.Join(path, file.Name())
		if file.IsDir() {
			readFilesAndTest(t, fp, skipList, tf)
		} else if skipFile(skipList, file.Name()) {
			continue
		} else {
			t.Run(fp, func(t *testing.T) {
				tf(t, fp)
			})
		}
	}
}

func loadFile(t *testing.T, path string) []byte {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func skipFile(skipList []string, fn string) bool {
	ion, _ := regexp.MatchString(`.ion$`, fn)
	bin, _ := regexp.MatchString(`.10n$`, fn)

	return !ion && !bin || isInSkipList(skipList, fn)
}

func isInSkipList(skipList []string, fn string) bool {
	for _, a := range skipList {
		if a == fn {
			return true
		}
	}
	return false
}

func writeToWriterFromReader(t *testing.T, r Reader, w Writer) {
	for r.Next() {
		name := r.FieldName()
		if name != "" {
			w.FieldName(name)
		}

		an := r.Annotations()
		if len(an) > 0 {
			w.Annotations(an...)
		}

		switch r.Type() {
		case NullType:
			err := w.WriteNull()
			if err != nil {
				t.Errorf("Something went wrong when writing Null value. " + err.Error())
			}

		case BoolType:
			val, err := r.BoolValue()
			if err != nil {
				t.Errorf("Something went wrong when reading Boolean value. " + err.Error())
			}
			err = w.WriteBool(val)
			if err != nil {
				t.Errorf("Something went wrong when writing Boolean value. " + err.Error())
			}

		case IntType:
			val, err := r.Int64Value()
			if err != nil {
				t.Errorf("Something went wrong when reading Int value. " + err.Error())
			}
			err = w.WriteInt(val)
			if err != nil {
				t.Errorf("Something went wrong when writing Int value. " + err.Error())
			}

		case FloatType:
			val, err := r.FloatValue()
			if err != nil {
				t.Errorf("Something went wrong when reading Float value. " + err.Error())
			}
			err = w.WriteFloat(val)
			if err != nil {
				t.Errorf("Something went wrong when writing Float value. " + err.Error())
			}

		case DecimalType:
			val, err := r.DecimalValue()
			if err != nil {
				t.Errorf("Something went wrong when reading Decimal value. " + err.Error())
			}
			err = w.WriteDecimal(val)
			if err != nil {
				t.Errorf("Something went wrong when writing Decimal value. " + err.Error())
			}

		case TimestampType:
			val, err := r.TimeValue()
			if err != nil {
				t.Errorf("Something went wrong when reading Timestamp value. " + err.Error())
			}
			err = w.WriteTimestamp(val)
			if err != nil {
				t.Errorf("Something went wrong when writing Timestamp value. " + err.Error())
			}

		case SymbolType:
			val, err := r.StringValue()
			if err != nil {
				t.Errorf("Something went wrong when reading Symbol value. " + err.Error())
			}
			err = w.WriteSymbol(val)
			if err != nil {
				t.Errorf("Something went wrong when writing Symbol value. " + err.Error())
			}

		case StringType:
			val, err := r.StringValue()
			if err != nil {
				t.Errorf("Something went wrong when reading String value. " + err.Error())
			}
			err = w.WriteString(val)
			if err != nil {
				t.Errorf("Something went wrong when writing String value. " + err.Error())
			}

		case ClobType:
			val, err := r.ByteValue()
			if err != nil {
				t.Errorf("Something went wrong when reading Clob value. " + err.Error())
			}
			err = w.WriteClob(val)
			if err != nil {
				t.Errorf("Something went wrong when writing Clob value. " + err.Error())
			}

		case BlobType:
			val, err := r.ByteValue()
			if err != nil {
				t.Errorf("Something went wrong when reading Blob value. " + err.Error())
			}
			err = w.WriteBlob(val)
			if err != nil {
				t.Errorf("Something went wrong when writing Blob value. " + err.Error())
			}

		case SexpType:
			r.StepIn()
			w.BeginSexp()
			writeToWriterFromReader(t, r, w)
			r.StepOut()
			w.EndSexp()

		case ListType:
			r.StepIn()
			w.BeginList()
			writeToWriterFromReader(t, r, w)
			r.StepOut()
			w.EndList()

		case StructType:
			r.StepIn()
			w.BeginStruct()
			writeToWriterFromReader(t, r, w)
			r.StepOut()
			w.EndStruct()
		}
	}

	if r.Err() != nil {
		t.Errorf(r.Err().Error())
	}
}
