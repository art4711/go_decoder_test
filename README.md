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
So I figured I would try `io.ReadAt` to get my byte slice. Yup, even
better.

### sl - sqlite3

Just put all values into a one-column table in Sqlite3. Fetch all rows
(sorted). I'm expecting this benchmark to run on a geological time
scale.

### bx - more or less equivalent to bi

Just to study the code of encoding/binary I extracted the relevant
parts of its code. This performs more or less the same as bi which
is a good indication that I found the right code.

### by - various experiments on how to make bx faster

Here I performed experiments to see how to make encoding/binary
faster. Trying to get the reflection based code fast failed,
everything I tried gave at most 50% speed improvements (like not
trying to figure out the type of the value of every slice element over
and over and over again). So let's see how encoding/binary would
perform if floats had a fast path just like ints.

## Results.

The test is done on a relatively new MacBook Pro. The disk (SSD) is
irrelevant because we're not timing writing and since all the data is
generated on test startup it will be in the buffer cache during the
test so we shouldn't be hitting the disk at all (if we do then there's
so much load on the machine that the test results are invalid anyway).

(rounding to 2 significant digits below)

encoding | consumption speed | file size |
---------|-------------------|-----------|
bi	| 120 MB/s	| 4MB	|
js	| 8.0 MB/s	| 11MB	| 
jd	| 6.0 MB/s	| 4.5MB | 
fm	| 3400 MB/s	| 4MB	| 
gb	| 90 MB/s	| 5.9MB |
bc	| 624 MB/s	| 4MB	|
ba	| 1367 MB/s	| 4MB	| 
sl	| 2.6 MB/s	| 18MB	|
bx	| 130 MB/s	| 4MB	|
by	| 540 MB/s	| 4MB	|

Raw data from one test run:

    $ go test -bench .
    PASS
    BenchmarkReadBinary	      50	  35737438 ns/op	 117.36 MB/s	 8388690 B/op	       4 allocs/op
    BenchmarkReadJSON	       2	 525182241 ns/op	   7.99 MB/s	54173740 B/op	 1026677 allocs/op
    BenchmarkReadJDef	       2	 696049978 ns/op	   6.03 MB/s	54236860 B/op	 1027243 allocs/op
    BenchmarkReadFmap	    1000	   1227830 ns/op	3416.03 MB/s	      32 B/op	       1 allocs/op
    BenchmarkReadGob	      50	  51869066 ns/op	  80.86 MB/s	10384991 B/op	     320 allocs/op
    BenchmarkReadBrutal	     200	   6716936 ns/op	 624.44 MB/s	16775281 B/op	      15 allocs/op
    BenchmarkReadBrutalA     500	   3066435 ns/op	1367.81 MB/s	 4194304 B/op	       1 allocs/op
    BenchmarkReadSqlite3       1	1596527775 ns/op	   2.63 MB/s	71190440 B/op	 2097197 allocs/op
    BenchmarkReadBinx	     100	  32981009 ns/op	 127.17 MB/s	 8388673 B/op	       4 allocs/op
    BenchmarkReadBiny	     200	   7741075 ns/op	 541.82 MB/s	 8388651 B/op	       3 allocs/op

### Additional result.

An C program equivalent to `ba` performs only 1.5x faster (around
2GB/s), so the problem is not in the I/O subsystem in Go, it's all in
the decoding and encoding. Something equivalent to fmap performs 1.2x
faster. Except if the mmap:ed array isn't `volatile` and we don't use
the sum which allows the compiler to optimize away everything and then
it actually performs 1132712x faster.

## Conclusion

Of the idiomatic ways of decoding things raw binary is the fastest as
expected. gob isn't that far behind which is good news because that
gives us type safety without ruining performance (compared to doing it
entirely raw). JSON is slow even though I didn't expect it to be so
catastrophically slow. Compressed JSON is even slower (tested to rule
out file size that could be the reason for JSON to be slow).

What surprised me the most was how sqlite3 didn't perform that bad at
all. It's the worst possible database schema and we still a third of
the performance of JSON. Shoving multiple values into the same row did
speed it up by a factor of 2 or so, I suspect the cost is actually in
the reflections in rows.Scan, not the database itself.

Breaking the rules of course gives us better performance, but I didn't
expect it to be so much better. Even the raw encoding/binary is 6
times slower than the brutal cast that does the same amount of file
I/O and should do the same amount of memory allocations (but somehow
does twice (I figured it out, ioutil.ReadAll does something stupid,
io.ReadAt doesn't, reflected in the `ba` test)). Mmap and a brutal
cast, of course, outperforms everything by a wide margin, as it should
be.

I understand that modern systems are all about sending JSON over HTTP,
but that's no excuse for a systems programming language to be so far
away from decent performance when it comes to reading raw data.

The most hilarious thing was when I dug even deeper in `by`. We go
through all the effort to properly decode the data through
endian-independent wrappers so that we don't switch on the endianness
of the machine we're running on, all is done with type safety and how
modern, pretty, well-written code should be written without any
assumptions about how memory works and then at the end (this is from
the math package):

    func Float32frombits(b uint32) float32 { return *(*float32)(unsafe.Pointer(&b)) }

So deep down, even the go programmers gave up at this level. Which
makes the whole effort completely wasted, we end up with a brutal cast
anyway. You can't abstract away data and memory. Your code is running
on a computer, not a mathematically and philosophically pure computer
science device. Sometimes you just need [to think about hex numbers
and their relationships with the operating system, the hardware, and
ancient blood
rituals](http://research.microsoft.com/en-us/people/mickens/thenightwatch.pdf).

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