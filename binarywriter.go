package ion

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"time"
)

// A binaryWriter writes binary ion.
type binaryWriter struct {
	writer
	bufs bufstack

	lst  SymbolTable
	lstb SymbolTableBuilder

	wroteLST bool
}

// NewBinaryWriter creates a new binary writer that will construct a
// local symbol table as it is written to.
func NewBinaryWriter(out io.Writer, sts ...SharedSymbolTable) Writer {
	return &binaryWriter{
		writer: writer{
			out: out,
		},
		lstb: NewSymbolTableBuilder(sts...),
	}
}

// NewBinaryWriterLST creates a new binary writer with a pre-built local
// symbol table.
func NewBinaryWriterLST(out io.Writer, lst SymbolTable) Writer {
	return &binaryWriter{
		writer: writer{
			out: out,
		},
		lst: lst,
	}
}

// Emit emits the given node. If we're currently at the top level, that
// means actually emitting to the output stream. If not, we emit append
// to the current bufseq.
func (w *binaryWriter) emit(node bufnode) error {
	s := w.bufs.peek()
	if s == nil {
		return node.EmitTo(w.out)
	}
	s.Append(node)
	return nil
}

// Write emits the given bytes as an atom.
func (w *binaryWriter) write(bs []byte) error {
	return w.emit(atom(bs))
}

// WriteTag writes out a type+length tag. Use me when you've already got the value to
// be written as a []byte and don't want to copy it.
func (w *binaryWriter) writeTag(code byte, len uint64) error {
	tl := tagLen(len)

	tag := make([]byte, tl)
	tag = appendTag(tag, code, len)

	return w.write(tag)
}

// WriteLST writes out a local symbol table.
func (w *binaryWriter) writeLST(lst SymbolTable) error {
	if err := w.write([]byte{0xE0, 0x01, 0x00, 0xEA}); err != nil {
		return err
	}
	return lst.WriteTo(w)
}

// BeginValue begins the process of writing a value by writing out
// its field name and annotations.
func (w *binaryWriter) beginValue() error {
	// We have to record/empty these before calling writeLST, which
	// will end up using/modifying them. Ugh.
	name := w.fieldName
	w.fieldName = ""
	tas := w.typeAnnotations
	w.typeAnnotations = nil

	// If we have a local symbol table and haven't written it out yet, do that now.
	if w.lst != nil && !w.wroteLST {
		w.wroteLST = true
		if err := w.writeLST(w.lst); err != nil {
			return err
		}
	}

	if w.InStruct() {
		if name == "" {
			return errors.New("ion: field name not set")
		}

		id, ok := w.lst.FindByName(name)
		if !ok {
			return fmt.Errorf("ion: symbol '%v' not defined", name)
		}
		if id < 0 {
			panic("negative id")
		}

		buf := make([]byte, 0, 10)
		buf = appendVarUint(buf, uint64(id))
		if err := w.write(buf); err != nil {
			return err
		}
	}

	if len(tas) > 0 {
		ids := make([]uint64, len(tas))
		idlen := uint64(0)

		for i, a := range tas {
			id, ok := w.lst.FindByName(a)
			if !ok {
				return fmt.Errorf("ion: symbol '%v' not defined", a)
			}
			if id < 0 {
				panic("negative id")
			}
			ids[i] = uint64(id)
			idlen += varUintLen(uint64(id))
		}

		buflen := idlen + varUintLen(idlen)
		buf := make([]byte, 0, buflen)

		buf = appendVarUint(buf, idlen)
		for _, id := range ids {
			buf = appendVarUint(buf, id)
		}

		w.bufs.push(&container{code: 0xE0})
		if err := w.write(buf); err != nil {
			return err
		}
	}

	return nil
}

// EndValue ends the process of writing a value by flushing it and its annotations
// up a level, if needed.
func (w *binaryWriter) endValue() error {
	seq := w.bufs.peek()
	if seq != nil {
		if c, ok := seq.(*container); ok && c.code == 0xE0 {
			w.bufs.pop()
			return w.emit(seq)
		}
	}
	return nil
}

// WriteValue writes an atomic value, invoking the given function to write the
// actual value contents.
func (w *binaryWriter) writeValue(f func() error) error {
	if err := w.beginValue(); err != nil {
		return err
	}

	if err := f(); err != nil {
		return err
	}

	return w.endValue()
}

// BeginContainer begins writing a new container.
func (w *binaryWriter) beginContainer(t ctxType, code byte) error {
	if err := w.beginValue(); err != nil {
		return err
	}

	w.ctx.push(t)
	w.bufs.push(&container{code: code})

	return nil
}

// EndContainer ends writing a container, emitting its buffered contents up
// a level in the stack.
func (w *binaryWriter) endContainer(t ctxType) error {
	if w.ctx.peek() != t {
		return errors.New("ion: not in that kind of container")
	}

	seq := w.bufs.peek()
	if seq != nil {
		w.bufs.pop()
		if err := w.emit(seq); err != nil {
			return err
		}
	}

	w.fieldName = ""
	w.typeAnnotations = nil
	w.ctx.pop()

	return w.endValue()
}

func (w *binaryWriter) WriteNull() {
	w.WriteNullWithType(NullType)
}

var nulls = func() []byte {
	ret := make([]byte, int(StructType)+1)
	ret[NoType] = 0x0F
	ret[NullType] = 0x0F
	ret[BoolType] = 0x1F
	ret[IntType] = 0x2F
	ret[FloatType] = 0x4F
	ret[DecimalType] = 0x5F
	ret[TimestampType] = 0x6F
	ret[SymbolType] = 0x7F
	ret[StringType] = 0x8F
	ret[ClobType] = 0x9F
	ret[BlobType] = 0xAF
	ret[ListType] = 0xBF
	ret[SexpType] = 0xCF
	ret[StructType] = 0xDF
	return ret
}()

func (w *binaryWriter) WriteNullWithType(t Type) {
	if w.err != nil {
		return
	}

	w.err = w.writeValue(func() error {
		return w.write([]byte{nulls[t]})
	})
}

func (w *binaryWriter) WriteBool(val bool) {
	if w.err != nil {
		return
	}

	w.err = w.writeValue(func() error {
		if val {
			return w.write([]byte{0x11})
		}
		return w.write([]byte{0x10})
	})
}

func (w *binaryWriter) WriteInt(val int64) {
	if w.err != nil {
		return
	}

	w.err = w.writeValue(func() error {
		if val == 0 {
			return w.write([]byte{0x20})
		}

		code := byte(0x20)
		mag := uint64(val)

		if val < 0 {
			code = 0x30
			mag = uint64(-val)
		}

		len := uintLen(mag)
		buflen := len + tagLen(len)

		buf := make([]byte, 0, buflen)
		buf = appendTag(buf, code, len)
		buf = appendUint(buf, mag)

		return w.write(buf)
	})
}

func (w *binaryWriter) WriteBigInt(val *big.Int) {
	if w.err != nil {
		return
	}

	w.err = w.writeValue(func() error {
		sign := val.Sign()
		if sign == 0 {
			return w.write([]byte{0x20})
		}

		code := byte(0x20)
		if sign < 0 {
			code = 0x30
		}

		bs := val.Bytes()

		if bl := uint64(len(bs)); bl < 64 {
			buflen := bl + tagLen(bl)
			buf := make([]byte, 0, buflen)

			buf = appendTag(buf, code, bl)
			buf = append(buf, bs...)
			return w.write(buf)
		}

		// no sense in copying, emit tag separately.
		if err := w.writeTag(code, uint64(len(bs))); err != nil {
			return err
		}
		return w.write(bs)
	})
}

func (w *binaryWriter) WriteFloat(val float64) {
	if w.err != nil {
		return
	}

	w.err = w.writeValue(func() error {
		if val == 0 {
			return w.write([]byte{0x40})
		}

		bs := make([]byte, 9)
		bs[0] = 0x48

		bits := math.Float64bits(val)
		binary.BigEndian.PutUint64(bs[1:], bits)

		return w.write(bs)
	})
}

func (w *binaryWriter) WriteDecimal(val *Decimal) {
	if w.err != nil {
		return
	}

	w.writeValue(func() error {
		coef, exp := val.CoEx()

		vlen := uint64(0)
		if exp != 0 {
			vlen += varIntLen(int64(exp))
		}
		if coef.Sign() != 0 {
			vlen += bigIntLen(coef)
		}

		buflen := vlen + tagLen(vlen)
		buf := make([]byte, 0, buflen)

		buf = appendTag(buf, 0x50, vlen)
		if exp != 0 {
			buf = appendVarInt(buf, int64(exp))
		}
		buf = appendBigInt(buf, coef)

		return w.write(buf)
	})
}

func (w *binaryWriter) WriteTimestamp(val time.Time) {
	if w.err != nil {
		return
	}

	w.err = w.writeValue(func() error {
		_, offset := val.Zone()
		offset /= 60
		utc := val.In(time.UTC)

		vlen := timeLen(offset, utc)
		buflen := vlen + tagLen(vlen)

		buf := make([]byte, 0, buflen)

		buf = appendTag(buf, 0x60, vlen)
		buf = appendTime(buf, offset, utc)

		return w.write(buf)
	})
}

func (w *binaryWriter) WriteSymbol(val string) {
	if w.err != nil {
		return
	}

	id := 0
	ok := true

	if w.lst != nil {
		id, ok = w.lst.FindByName(val)
		if !ok {
			w.err = fmt.Errorf("ion: symbol '%v' not defined in local symbol table", val)
			return
		}
	} else {
		id, _ = w.lstb.Add(val)
	}

	w.err = w.writeValue(func() error {
		vlen := uintLen(uint64(id))
		buflen := vlen + tagLen(vlen)
		buf := make([]byte, 0, buflen)

		buf = appendTag(buf, 0x70, vlen)
		buf = appendUint(buf, uint64(id))

		return w.write(buf)
	})
}

func (w *binaryWriter) WriteString(val string) {
	if w.err != nil {
		return
	}

	w.err = w.writeValue(func() error {
		if len(val) == 0 {
			return w.write([]byte{0x80})
		}

		vlen := uint64(len(val))
		buflen := vlen + tagLen(vlen)
		buf := make([]byte, 0, buflen)

		buf = appendTag(buf, 0x80, vlen)
		buf = append(buf, val...)

		return w.write(buf)
	})
}

func (w *binaryWriter) WriteClob(val []byte) {
	w.writeLob(0x90, val)
}

func (w *binaryWriter) WriteBlob(val []byte) {
	w.writeLob(0xA0, val)
}

func (w *binaryWriter) writeLob(code byte, val []byte) {
	if w.err != nil {
		return
	}

	w.err = w.writeValue(func() error {
		vlen := uint64(len(val))

		if vlen < 64 {
			buflen := vlen + tagLen(vlen)
			buf := make([]byte, 0, buflen)

			buf = appendTag(buf, code, vlen)
			buf = append(buf, val...)

			return w.write(buf)
		}

		if err := w.writeTag(code, vlen); err != nil {
			return err
		}
		return w.write(val)
	})
}

func (w *binaryWriter) WriteValue(val interface{}) {
	m := Encoder{
		w:        w,
		sortMaps: true,
	}
	w.err = m.Encode(val)
}

func (w *binaryWriter) BeginList() {
	if w.err != nil {
		return
	}
	w.err = w.beginContainer(ctxInList, 0xB0)
}

func (w *binaryWriter) EndList() {
	if w.err != nil {
		return
	}
	w.err = w.endContainer(ctxInList)
}

func (w *binaryWriter) BeginSexp() {
	if w.err != nil {
		return
	}
	w.err = w.beginContainer(ctxInSexp, 0xC0)
}

func (w *binaryWriter) EndSexp() {
	if w.err != nil {
		return
	}
	w.err = w.endContainer(ctxInSexp)
}

func (w *binaryWriter) BeginStruct() {
	if w.err != nil {
		return
	}
	w.err = w.beginContainer(ctxInStruct, 0xD0)
}

func (w *binaryWriter) EndStruct() {
	if w.err != nil {
		return
	}
	w.err = w.endContainer(ctxInStruct)
}

func (w *binaryWriter) Finish() error {
	if w.err != nil {
		return w.err
	}
	if w.ctx.peek() != ctxAtTopLevel {
		w.err = errors.New("ion: not at top level")
		return w.err
	}

	w.fieldName = ""
	w.typeAnnotations = nil
	w.wroteLST = false

	seq := w.bufs.peek()
	if seq != nil {
		w.bufs.pop()
		if w.bufs.peek() != nil {
			panic("at top level but too many bufseqs")
		}

		lst := w.lstb.Build()
		if err := w.writeLST(lst); err != nil {
			return err
		}
		if w.err = w.emit(seq); w.err != nil {
			return w.err
		}
	}

	return nil
}