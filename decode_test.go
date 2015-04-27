/*
 * Copyright (c) 2015 Artur Grabowski <art@blahonga.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */
package decode_test

import (
	"testing"
	"math/rand"
	"math"
	"encoding/binary"
	"encoding/json"
	"encoding/gob"
	"os"
	"fmt"
	"io"
	"io/ioutil"
	"github.com/art4711/filemap"
	"compress/flate"
	"unsafe"
	"reflect"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"errors"
)

const size = 1024*1024
const expected = float32(523902.12)

type tested interface {
	Generate(string, []float32)
	OpenReader(string) error
	Reset()
	ReadAndSum(testing.TB) float32
	Close()
}


type simpleFileOC struct {
	f *os.File
}

func (sf *simpleFileOC)OpenReader(fname string) error {
	f, err := os.Open(fname)
	sf.f = f
	return err
}

func (sf *simpleFileOC)Close() {
	sf.f.Close()
}

type simpleFile struct {
	simpleFileOC
}

func (sf *simpleFile)Reset() {
	sf.f.Seek(0, os.SEEK_SET)
}

type binFile struct {}

func (b *binFile)Generate(fname string, floatarr []float32) {
	file, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	err = binary.Write(file, binary.LittleEndian, floatarr)
	if err != nil {
		panic(err)
	}	
}

/*
 * File I/O with encoding/binary to read and write the data
 */
type bi struct {
	simpleFile
	binFile
}

func (bi *bi)ReadAndSum(tb testing.TB) float32 {
	floatarr := make([]float32, size)
	if err := binary.Read(bi.f, binary.LittleEndian, floatarr); err != nil {
		tb.Fatal(err)
	}
	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

/*
 * File I/O with encoding/json to read and write the data
 */
type js struct {
	simpleFile
}

func (js *js)Generate(fname string, floatarr []float32) {
	file, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.Encode(floatarr)
}

func (js *js)ReadAndSum(tb testing.TB) float32 {
	floatarr := make([]float32, size)
	decoder := json.NewDecoder(js.f)
	if err := decoder.Decode(&floatarr); err != nil {
		tb.Fatal(err)
	}
	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

/*
 * Deflated file I/O with encoding/json to read and write the data
 */
type jd struct {
	simpleFile
}

func (jd *jd)Generate(fname string, floatarr []float32) {
	file, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	wr, err := flate.NewWriter(file, flate.DefaultCompression)
	if err != nil {
		panic(err)
	}
	defer wr.Close()
	
	encoder := json.NewEncoder(wr)
	encoder.Encode(floatarr)
}

func (jd *jd)ReadAndSum(tb testing.TB) float32 {
	floatarr := make([]float32, size)
	r := flate.NewReader(jd.f)
	defer r.Close()
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&floatarr); err != nil {
		tb.Fatal(err)
	}
	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

/*
 * mmap:ed file I/O with brutal casting to read the data.
 */
type fm struct {
	fmap *filemap.Map
	binFile
}

func (fm *fm)ReadAndSum(tb testing.TB) float32 {
	sl, err := fm.fmap.Slice(4 * size, 0, size);
	if err != nil {
		tb.Fatal(err)
	}
	floatarr := *(*[]float32)(sl)

	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

func (fm *fm)OpenReader(fname string) error {
	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	fm.fmap, err = filemap.NewReader(f)
	return err
}

func (fm *fm)Reset() {
	// no need to do anything.
}

func (fm *fm)Close() {
	fm.fmap.Close()
}

/*
 * ioutil.ReadAll with brutal casting to read the data.
 */
type bc struct {
	simpleFile
	binFile
}

func (bc *bc)ReadAndSum(tb testing.TB) float32 {
	b, err := ioutil.ReadAll(bc.f)
	if err != nil {
		tb.Fatal(err)
	}

	bsl := (*reflect.SliceHeader)(unsafe.Pointer(&b))

	fsl := *bsl
	fsl.Len /= 4
	fsl.Cap /= 4
	floatarr := *(*[]float32)(unsafe.Pointer(&fsl))

	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

/*
 * File I/O with ReadAt and brutal casting to read the data.
 */
type ba struct {
	simpleFileOC
	binFile
}

func (ba *ba)ReadAndSum(tb testing.TB) float32 {
	b := make([]byte, size * 4)
	n, err := ba.f.ReadAt(b, 0)
	if err != nil {
		tb.Fatal(err)
	}
	if n != size * 4 {
		tb.Fatalf("readat: %v", n)
	}

	bsl := (*reflect.SliceHeader)(unsafe.Pointer(&b))

	fsl := *bsl
	fsl.Len /= 4
	fsl.Cap /= 4
	floatarr := *(*[]float32)(unsafe.Pointer(&fsl))

	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

func (ba *ba)Reset() {
}

/*
 * File I/O with encoding/gob to read and write the data
 */
type gb struct {
	simpleFile
}

func (gb *gb)Generate(fname string, floatarr []float32) {
	file, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	encoder.Encode(floatarr)
}

func (gb *gb)ReadAndSum(tb testing.TB) float32 {
	floatarr := make([]float32, size)
	decoder := gob.NewDecoder(gb.f)
	if err := decoder.Decode(&floatarr); err != nil {
		tb.Fatal(err)
	}
	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

/*
 * File I/O with more or less the same thing that encoding/binary does to read and write the data
 */
type bx struct {
	simpleFile
	binFile
}

type bxcoder struct {
	buf []byte
}

func (d *bxcoder) uint32() uint32 {
	x := uint32(d.buf[0]) | uint32(d.buf[1]) << 8 | uint32(d.buf[2]) << 16 | uint32(d.buf[3]) << 24
	d.buf = d.buf[4:]
	return x
}

func (d *bxcoder)value(v reflect.Value) {
	switch v.Kind() {
	case reflect.Slice:
		l := v.Len()
		for i := 0; i < l; i++ {
			d.value(v.Index(i))
		}
	case reflect.Float32:
		v.SetFloat(float64(math.Float32frombits(d.uint32())))
	default:
		panic("this is not a generic implementation")
	}
}


func (bx *bx)ReadAndSum(tb testing.TB) float32 {
	floatarr := make([]float32, size)
	v := reflect.ValueOf(floatarr)
	sz := v.Len() * int(v.Type().Elem().Size())

	bxc := &bxcoder{}
	bxc.buf = make([]byte, sz)
	if _, err := io.ReadFull(bx.f, bxc.buf); err != nil {
		tb.Fatal(err)
	}
	bxc.value(v)

	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

/*
 * Same as bx, but let's try to be smart about it.
 */

type by struct {
	simpleFile
	binFile
}

func intDataSize(data interface{}) int {
	switch data := data.(type) {
	case []float32:
		return 4 * len(data)
	}
	return 0
}

func u32(b []byte) uint32 {
	return (uint32(b[0]) | uint32(b[1]) << 8 | uint32(b[2]) << 16 | uint32(b[3]) << 24)
}

func (by *by)read(r io.Reader, data interface{}) error {
        if n := intDataSize(data); n != 0 {
                var b [8]byte
                var bs []byte
                if n > len(b) {
                        bs = make([]byte, n)
                } else {
                        bs = b[:n]
                }
                if _, err := io.ReadFull(r, bs); err != nil {
                        return err
                }
		switch data := data.(type) {
		case []float32:
			for i := range data {
				data[i] = math.Float32frombits(u32(bs[4*i:]))
			}
		}
		return nil
	}
	return errors.New("not reached, please")
}


func (by *by)ReadAndSum(tb testing.TB) float32 {
	floatarr := make([]float32, size)

	err := by.read(by.f, floatarr)
	if err != nil {
		tb.Fatal(err)
	}

	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

/*
 * Sqlite3 dumb storage.
 */
type sl struct {
	db *sql.DB
	stmt *sql.Stmt
}

func (sl *sl)Generate(fname string, floatarr []float32) {
	os.Remove(fname)
	db, err := sql.Open("sqlite3", fname)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	cStmt := "create table foo (id integer not null primary key, value float)"
	_, err = db.Exec(cStmt)
	if err != nil {
		panic(err)
	}

	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	stmt, err := tx.Prepare("insert into foo(value) values (?)")
	if err != nil {
		panic(err)
	}
	for _, v := range floatarr {
		if _, err := stmt.Exec(v); err != nil {
			panic(err)
		}
	}
	tx.Commit()
	stmt.Close()

}

func (sl *sl)OpenReader(fname string) error {
	db, err := sql.Open("sqlite3", fname)
	if err != nil {
		return err
	}
	sl.db = db
	sl.stmt, err = sl.db.Prepare("select value from foo order by id")
	if err != nil {
		panic(err)
	}
	return err
}

func (sl *sl)ReadAndSum(tb testing.TB) float32 {
	rows, err := sl.stmt.Query()
	if err != nil {
		tb.Fatal(err)
	}
	defer rows.Close()

	floatarr := make([]float32, size)
	i := 0
	for rows.Next() {
		err = rows.Scan(&floatarr[i])
		if err != nil {
			tb.Fatal(err)
		}
		i++
	}

	if i != size {
		tb.Fatal(fmt.Errorf("rows mismatch: %d != %d", i, size));
	}

	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

func (sl *sl)Reset() {
}

func (sl *sl)Close() {
	sl.db.Close()
}

type mm struct {
	fa []float32
}

func (mm *mm) Generate(fname string, floatarr []float32) {
	mm.fa = floatarr
}

func (mm *mm) OpenReader(string) error {
	return nil
}

func (mm *mm) Reset() {
}

func (mm *mm) ReadAndSum(testing.TB) float32 {
	s := float32(0)
	for _, v := range mm.fa {
		s += v
	}
	return s
}

func (mm *mm) Close() {
}


const (
	T_BI = iota
	T_JS
	T_JD
	T_FM
	T_GB
	T_BC
	T_BA
	T_SL
	T_BX
	T_BY
	T_MM
)

type tt struct{
	tt tested
	fname string
}
var toTest = [...]tt{
	T_BI: { &bi{}, "float-file.bin" },
	T_JS: { &js{}, "float-file.json" },
	T_JD: { &jd{}, "float-file.json.z" },
	T_FM: { &fm{}, "float-file.fm" },
	T_GB: { &gb{}, "float-file.gob" },
	T_BC: { &bc{}, "float-file.bc" },
	T_BA: { &ba{}, "float-file.ba" },
	T_SL: { &sl{}, "float-file.sl" },
	T_BX: { &bx{}, "float-file.bx" },
	T_BY: { &by{}, "float-file.by" },
	T_MM: { &mm{}, "" },
}

/* We're not testing encoding, just decoding. */
func init() {
	floatarr := [size]float32{}
	rand.Seed(4711)
	for i := range floatarr {
		floatarr[i] = rand.Float32()
	}

	for _, te := range toTest {
		te.tt.Generate(te.fname, floatarr[:])
	}
}

func genericBenchmark(b *testing.B, which int) {
	te := toTest[which]

	b.ReportAllocs()
	err := te.tt.OpenReader(te.fname)
	if err != nil {
		b.Fatal(err)
	}
	defer te.tt.Close()
	for t := 0; t < b.N; t++ {
		te.tt.Reset()
		te.tt.ReadAndSum(b)
		b.SetBytes(size * 4)
	}
}

func BenchmarkReadBinary(b *testing.B) {
	genericBenchmark(b, T_BI)
}

func BenchmarkReadJSON(b *testing.B) {
	genericBenchmark(b, T_JS)
}

func BenchmarkReadJDef(b *testing.B) {
	genericBenchmark(b, T_JD)
}

func BenchmarkReadFmap(b *testing.B) {
	genericBenchmark(b, T_FM)
}

func BenchmarkReadGob(b *testing.B) {
	genericBenchmark(b, T_GB)
}

func BenchmarkReadBrutal(b *testing.B) {
	genericBenchmark(b, T_BC)
}

func BenchmarkReadBrutalA(b *testing.B) {
	genericBenchmark(b, T_BA)
}

func BenchmarkReadSqlite3(b *testing.B) {
	genericBenchmark(b, T_SL)
}

func BenchmarkReadBinx(b *testing.B) {
	genericBenchmark(b, T_BX)
}

func BenchmarkReadBiny(b *testing.B) {
	genericBenchmark(b, T_BY)
}

func BenchmarkReadMM(b *testing.B) {
	genericBenchmark(b, T_MM)
}

func genericTest(t *testing.T, which int) {
	te := toTest[which]
	err := te.tt.OpenReader(te.fname)
	if err != nil {
		t.Fatal(err)
	}
	defer te.tt.Close()
	s := te.tt.ReadAndSum(t)
	if math.Abs(float64(s - expected)) > 0.005 {
		t.Fatalf("%v != %v, did the pseudo-random generator change?", s, expected)
	}
}

func TestSumBinary(t *testing.T) {
	genericTest(t, T_BI)
}

func TestSumJSON(t *testing.T) {
	genericTest(t, T_JS)
}

func TestSumJDef(t *testing.T) {
	genericTest(t, T_JD)
}

func TestSumFmap(t *testing.T) {
	genericTest(t, T_FM)
}

func TestSumGob(t *testing.T) {
	genericTest(t, T_GB)
}

func TestSumBrutal(t *testing.T) {
	genericTest(t, T_BC)
}

func TestSumBrutalA(t *testing.T) {
	genericTest(t, T_BA)
}

func TestSumSqlite3(t *testing.T) {
	genericTest(t, T_SL)
}

func TestSumBinx(t *testing.T) {
	genericTest(t, T_BX)
}

func TestSumBiny(t *testing.T) {
	genericTest(t, T_BY)
}

func TestSumMM(t *testing.T) {
	genericTest(t, T_MM)
}
