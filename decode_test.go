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
	"io/ioutil"
	"github.com/art4711/filemap"
	"compress/flate"
	"unsafe"
	"reflect"
)

const size = 1024*1024
const expected = float32(523902.12)

type tested interface {
	Generate([]float32)
	OpenReader() error
	Reset()
	ReadAndSum(testing.TB) float32
	Close()
}

/*
 * File I/O with encoding/binary to read and write the data
 */
type bi struct {
	fname string
	f *os.File
}

func rawBinaryGenerate(floatarr []float32, fname string) {
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

func (bi *bi)Generate(floatarr []float32) {
	rawBinaryGenerate(floatarr, bi.fname)
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

func (bi *bi) OpenReader() error {
	f, err := os.Open(bi.fname)
	bi.f = f
	return err
}

func (bi *bi)Reset() {
	bi.f.Seek(0, os.SEEK_SET)
}

func (bi *bi)Close() {
	bi.f.Close()
}

/*
 * File I/O with encoding/json to read and write the data
 */
type js struct {
	fname string
	f *os.File
}

func (js *js)Generate(floatarr []float32) {
	file, err := os.OpenFile(js.fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
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

func (js *js)OpenReader() error {
	f, err := os.Open(js.fname)
	js.f = f
	return err
}

func (js *js)Reset() {
	js.f.Seek(0, os.SEEK_SET)
}

func (js *js)Close() {
	js.f.Close()
}

/*
 * Deflated file I/O with encoding/json to read and write the data
 */
type jd struct {
	fname string
	f *os.File
}

func (jd *jd)Generate(floatarr []float32) {
	file, err := os.OpenFile(jd.fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
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

func (jd *jd)OpenReader() error {
	f, err := os.Open(jd.fname)
	jd.f = f
	return err
}

func (jd *jd)Reset() {
	jd.f.Seek(0, os.SEEK_SET)
}

func (jd *jd)Close() {
	jd.f.Close()
}

/*
 * mmap:ed file I/O with brutal casting to read the data.
 */
type fm struct {
	fname string
	fmap *filemap.Map
}

func (fm *fm)Generate(floatarr []float32) {
	rawBinaryGenerate(floatarr, fm.fname)
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

func (fm *fm)OpenReader() error {
	f, err := os.Open(fm.fname)
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
	fname string
	f *os.File
}

func (bc *bc)Generate(floatarr []float32) {
	rawBinaryGenerate(floatarr, bc.fname)
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

func (bc *bc)OpenReader() error {
	f, err := os.Open(bc.fname)
	bc.f = f
	return err
}

func (bc *bc)Reset() {
	bc.f.Seek(0, os.SEEK_SET)
}

func (bc *bc)Close() {
	bc.f.Close()
}

/*
 * File I/O with ReadAt and brutal casting to read the data.
 */
type ba struct {
	fname string
	f *os.File
}

func (ba *ba)Generate(floatarr []float32) {
	rawBinaryGenerate(floatarr, ba.fname)
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

func (ba *ba)OpenReader() error {
	f, err := os.Open(ba.fname)
	ba.f = f
	return err
}

func (ba *ba)Reset() {
	ba.f.Seek(0, os.SEEK_SET)
}

func (ba *ba)Close() {
	ba.f.Close()
}

/*
 * File I/O with encoding/gob to read and write the data
 */
type gb struct {
	fname string
	f *os.File
}

func (gb *gb)Generate(floatarr []float32) {
	file, err := os.OpenFile(gb.fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
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

func (gb *gb)OpenReader() error {
	f, err := os.Open(gb.fname)
	gb.f = f
	return err
}

func (gb *gb)Reset() {
	gb.f.Seek(0, os.SEEK_SET)
}

func (gb *gb)Close() {
	gb.f.Close()
}

var (
	binTest = &bi{fname: "float-file.bin"}
	jsonTest = &js{fname: "float-file.json"}
	jsonDeflateTest = &jd{fname: "float-file.json.z"}
	fileMapTest = &fm{fname: "float-file.fm"}
	gobTest = &gb{fname: "float-file.gob"}
	bcTest = &bc{fname: "float-file.bc"}
	baTest = &ba{fname: "float-file.ba"}
)

var toTest = [...]tested{ binTest, jsonTest, jsonDeflateTest, fileMapTest, gobTest, bcTest, baTest }

/* We're not testing encoding, just decoding. */
func init() {
	floatarr := [size]float32{}
	rand.Seed(4711)
	for i := range floatarr {
		floatarr[i] = rand.Float32()
	}

	for _, t := range toTest {
		t.Generate(floatarr[:])
	}
}

func genericBenchmark(b *testing.B, te tested) {
	b.ReportAllocs()
	err := te.OpenReader()
	if err != nil {
		b.Fatal(err)
	}
	defer te.Close()
	for t := 0; t < b.N; t++ {
		te.Reset()
		te.ReadAndSum(b)
		b.SetBytes(size * 4)
	}
}

func BenchmarkReadBinary(b *testing.B) {
	genericBenchmark(b, binTest)
}

func BenchmarkReadJSON(b *testing.B) {
	genericBenchmark(b, jsonTest)
}

func BenchmarkReadJDef(b *testing.B) {
	genericBenchmark(b, jsonDeflateTest)
}

func BenchmarkReadFmap(b *testing.B) {
	genericBenchmark(b, fileMapTest)
}

func BenchmarkReadGob(b *testing.B) {
	genericBenchmark(b, gobTest)
}

func BenchmarkReadBrutal(b *testing.B) {
	genericBenchmark(b, bcTest)
}

func BenchmarkReadBrutalA(b *testing.B) {
	genericBenchmark(b, baTest)
}

func genericTest(t *testing.T, te tested) {
	err := te.OpenReader()
	if err != nil {
		t.Fatal(err)
	}
	defer te.Close()
	s := te.ReadAndSum(t)
	if math.Abs(float64(s - expected)) > 0.005 {
		t.Fatalf("%v != %v, did the pseudo-random generator change?", s, expected)
	}
}

func TestSumBinary(t *testing.T) {
	genericTest(t, binTest)
}

func TestSumJSON(t *testing.T) {
	genericTest(t, jsonTest)
}

func TestSumJDef(t *testing.T) {
	genericTest(t, jsonDeflateTest)
}

func TestSumFmap(t *testing.T) {
	genericTest(t, fileMapTest)
}

func TestSumGob(t *testing.T) {
	genericTest(t, gobTest)
}

func TestSumBrutal(t *testing.T) {
	genericTest(t, bcTest)
}

func TestSumBrutalA(t *testing.T) {
	genericTest(t, baTest)
}

