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
	"io"
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
	OpenReader() (io.ReadCloser, error)
	Reset(io.Reader)
	ReadAndSum(io.Reader, testing.TB) float32
}

/*
 * File I/O with encoding/binary to read and write the data
 */
type bi string

func (b bi)Generate(floatarr []float32) {
	file, err := os.OpenFile(string(b), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	err = binary.Write(file, binary.LittleEndian, floatarr)
	if err != nil {
		panic(err)
	}	
}

func (b bi)ReadAndSum(f io.Reader, tb testing.TB) float32 {
	floatarr := make([]float32, size)
	if err := binary.Read(f, binary.LittleEndian, floatarr); err != nil {
		tb.Fatal(err)
	}
	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

func (b bi) OpenReader() (io.ReadCloser, error) {
	return os.Open(string(b))
}

func (b bi)Reset(f io.Reader) {
	file := f.(*os.File)
	file.Seek(0, os.SEEK_SET)
}

/*
 * File I/O with encoding/json to read and write the data
 */
type js string

func (js js)Generate(floatarr []float32) {
	file, err := os.OpenFile(string(js), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.Encode(floatarr)
}

func (j js)ReadAndSum(f io.Reader, tb testing.TB) float32 {
	floatarr := make([]float32, size)
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&floatarr); err != nil {
		tb.Fatal(err)
	}
	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

func (j js)OpenReader() (io.ReadCloser, error) {
	return os.Open(string(j))
}

func (j js)Reset(f io.Reader) {
	file := f.(*os.File)
	file.Seek(0, os.SEEK_SET)
}

/*
 * Deflated file I/O with encoding/json to read and write the data
 */
type jd string

func (jd jd)Generate(floatarr []float32) {
	file, err := os.OpenFile(string(jd), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
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

func (j jd)ReadAndSum(f io.Reader, tb testing.TB) float32 {
	floatarr := make([]float32, size)
	r := flate.NewReader(f)
	r.Close()
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

func (j jd)OpenReader() (io.ReadCloser, error) {
	return os.Open(string(j))
}

func (j jd)Reset(f io.Reader) {
	file := f.(*os.File)
	file.Seek(0, os.SEEK_SET)
}

/*
 * mmap:ed file I/O with brutal casting to read the data.
 */
type fm string

func (fname fm)Generate(floatarr []float32) {
	bi(fname).Generate(floatarr)
}

func (fm fm)ReadAndSum(f io.Reader, tb testing.TB) float32 {
	fmap, err := filemap.NewReader(f.(*os.File));
	if err != nil {
		tb.Fatal(err)
	}
	defer fmap.Close()

	sl, err := fmap.Slice(4 * size, 0, size);
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

func (fm fm)OpenReader() (io.ReadCloser, error) {
	return os.Open(string(fm))
}

func (fm fm)Reset(f io.Reader) {
	// no need to do anything.
}

/*
 * File I/O with brutal casting to read the data.
 */
type bc string

func (bc bc)Generate(floatarr []float32) {
	bi(bc).Generate(floatarr)
}

func (bc bc)ReadAndSum(f io.Reader, tb testing.TB) float32 {
	b, err := ioutil.ReadAll(f)
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

func (bc bc)OpenReader() (io.ReadCloser, error) {
	return os.Open(string(bc))
}

func (bc bc)Reset(f io.Reader) {
	file := f.(*os.File)
	file.Seek(0, os.SEEK_SET)
}

/*
 * File I/O with encoding/gob to read and write the data
 */
type gb string

func (gb gb)Generate(floatarr []float32) {
	file, err := os.OpenFile(string(gb), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	encoder.Encode(floatarr)
}

func (gb gb)ReadAndSum(f io.Reader, tb testing.TB) float32 {
	floatarr := make([]float32, size)
	decoder := gob.NewDecoder(f)
	if err := decoder.Decode(&floatarr); err != nil {
		tb.Fatal(err)
	}
	s := float32(0)
	for _, v := range floatarr {
		s += v
	}
	return s
}

func (gb gb)OpenReader() (io.ReadCloser, error) {
	return os.Open(string(gb))
}

func (gb gb)Reset(f io.Reader) {
	file := f.(*os.File)
	file.Seek(0, os.SEEK_SET)
}

var (
	binTest = bi("float-file.bin")
	jsonTest = js("float-file.json")
	jsonDeflateTest = jd("float-file.json.z")
	fileMapTest = fm("float-file.fm")
	gobTest = gb("float-file.gob")
	bcTest = bc("float-file.bc")
)

var toTest = [...]tested{ binTest, jsonTest, jsonDeflateTest, fileMapTest, gobTest, bcTest }

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
	file, err := te.OpenReader()
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	for t := 0; t < b.N; t++ {
		te.Reset(file)
		te.ReadAndSum(file, b)
		b.SetBytes(size * 4)
	}
	file.Close()
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

func genericTest(t *testing.T, te tested) {
	file, err := te.OpenReader()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	s := te.ReadAndSum(file, t)
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

