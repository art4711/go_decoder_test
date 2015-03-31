# Performance of decoding stored data in Go.

Tests to see the performance of various ways to retreive data in Go.

## Test setup:

### Choices

1. I don't care about encoding at this moment. Imagine a file
pre-generated at some point in time that we need to read efficiently
at run-time.

2. I store floats because at least encoding/binary has optimizations
for integers. So imagine this is a game that wants to store vertex
data.

### Details

What we do is to generate an array of 1 million (not at all) random
float32 numbers and store them in a file, the test tests that what we
get is at least close to what is expected, then the benchmarks test
decoding all the floating point values into an array (and make a sum
of that array, because why not).

## Encodings:

### bi - normal file I/O with encoding/binary

Reads the file through `binary.Read`, couldn't be simpler.

### js - normal file I/O with encoding/json

Reads the file through `json.Decode`.

### jd - deflated file I/O with encoding/json

Reads the file through `flate.Reader` and `json.Decode`. Added to see
if the performance of json was caused by the larger amount of data
that needs to be read (it wasn't).

### gb - normal file I/O with encoding/gob

Reader the file through `gob.Decode`. Maybe the standard wire format
performs better (spoiler: it doesn't).

### bc - ioutil.ReadALL I/O with brutal casting

Reads the whole file with `ioutil.ReadAll`, takes the byte slice,
casts it to `reflect.SliceHeader`, copies the slice header,
performs some brain surgery on it, casts it to our expected slice.

### fm - mmap of file and hand-crafting a []float32 slice

Let's see how things perfom when we can use modern (30 years old)
memory management facilities of an operating system and don't need
to copy data back and forth.

### ba - io.ReadAt I/O with brutal casting

I didn't like that bc was doing more memory allocations than expected.
So I figured I would try `io.ReadAt` to get my byte slice. For some
reason This is faster than mmap and the (what I though was) equivalent
C code. How the hell is ReadAt implemented?

## Results.

The test is done on a relatively new MacBook Pro. The disk (SSD) is
irrelevant because we're not timing writing and since all the data is
generated on test startup it will be in the buffer cache during the
test so we shouldn't be hitting the disk at all (if we do then there's
so much load on the machine that the test results are invalid anyway).

(creative rounding below)

encoding | consumption speed | file size | how much slower than best
---------|-------------------|-----------|--------------
bi	| 130 MB/s	| 4MB	| 14x
js	| 9.2 MB/s	| 11MB | 200x
jd	| 6.8 MB/s	| 4.5MB | 260x
gb	| 90 MB/s	| 5.9MB | 20x
bc	| 890 MB/s	| 4MB | 2x
fm	| 1800 MB/s	| 4MB | 1x
ba	| 1800 MB/s	| 4MB | 1x

Raw data from one test run:

    $ go test -bench .
    PASS
    BenchmarkReadBinary	      50	  31304283 ns/op	 133.98 MB/s	 8388691 B/op	       4 allocs/op
    BenchmarkReadJSON	       5	 456980729 ns/op	   9.18 MB/s	54173700 B/op	 1026675 allocs/op
    BenchmarkReadJDef	       2	 614027465 ns/op	   6.83 MB/s	54236812 B/op	 1027243 allocs/op
    BenchmarkReadFmap	    1000	   2315159 ns/op	1811.67 MB/s	     273 B/op	       4 allocs/op
    BenchmarkReadGob	      50	  46407956 ns/op	  90.38 MB/s	10384992 B/op	     320 allocs/op
    BenchmarkReadBrutal	     500	   4716730 ns/op	 889.24 MB/s	16775280 B/op	      15 allocs/op
    BenchmarkReadBrutalA	    1000	   2306147 ns/op	1818.75 MB/s	 4194304 B/op	       1 allocs/op

### Additional result.

An C program equivalent to bc performs only 1.2x faster (slightly
above 1GB/s), so the problem is not in the I/O subsystem in Go, it's
all in the decoding and encoding.

## Conclusion

Of the idiomatic ways of decoding things raw binary is the fastest as
expected. gob isn't that far behind which is good news because that
gives us type safety without ruining performance (compared to doing it
entirely raw). JSON is slow even though I didn't expect it to be so
catastrophically slow. Compressed JSON is even slower (tested to rule
out file size that could be the reason for JSON to be slow).

Breaking the rules of course gives us better performance, but I didn't
expect it to be so much better. Even the raw encoding/binary is 6
times slower than the brutal cast that does the same amount of file
I/O and should do the same amount of memory allocations (but somehow
does twice).

I understand that modern systems are all about sending JSON over HTTP,
but that's no excuse for a systems programming language to be so far
away from decent performance when it comes to reading raw data.

## Code

The test code in [decode_test.go](decode_test.go) should be simple
enough to understand. I went a bit wild abstracting things, but it
should be clear what part does what. The filemap code it depends on
may or may not work on any operating system other than MacOS because
there is some type screwup going on in CGO. It should be trivial to
figure it out (and even more trivial to comment out that test).

The C code to do the same thing is in
[decode_test.c.go_test_picks_this_up](decode_test.c.go_test_picks_this_up)
named that way because `go test` wants to compile c files for some
reason.  It uses [my stopwatch code](https://github.com/art4711/timing).
It's written as lazily as I could get away with and wants a file
generated by first running go test.