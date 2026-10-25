# Debug Printing

A program can print text and values for debugging. It writes to two addresses
in the unmapped range `$4018` to `$401F`. Real hardware ignores these writes, so
a ROM with debug output also runs on a console.

| Address | Name | Data |
|---------|------|------|
| `$401B` | Text port | One text character for each write |
| `$401C` | Value port | Value records that the emulator formats |

The emulator writes the output to stderr, so the output stays separate from
the trace output on stdout. It writes each line when it receives a newline
character. It writes an unfinished line when the program stops.

```bash
nesgoemu -c game.nes 2> printf.log
```

Use the `-no-printf` flag to ignore the writes.

## Text Port

The program writes one character at a time to `$401B`. The emulator does not
change the text. A percent sign is a usual character.

The llvm-mos SDK uses this address for `stdout`, and the Mesen script
[`printf-mesen.lua`](https://github.com/llvm-mos/llvm-mos-sdk/blob/main/utils/printf-mesen.lua)
reads it. A program that uses only the text port gives the same output in
these tools.

## Value Port

The program writes a value record to `$401C`. The emulator formats the value
with the Go `fmt` package and adds the text to the current line. The output is
the same as the output of the Go verb on the host.

A record has these parts, in this sequence:

1. Optional flags, width, and precision of a Go verb, for example `04`. The
   permitted characters are `+-# 0123456789.`. The maximum is 8 characters.
2. One type letter. The letter sets the source and the Go type of the value.
3. The Go verb, for example `d` or `x`.
4. The argument bytes of the type, low byte first.

| Type letter | Argument bytes | Value |
|-------------|----------------|-------|
| `u` | one byte | `uint8` |
| `i` | one byte | `int8` |
| `U` | low byte, high byte | `uint16` |
| `I` | low byte, high byte | `int16` |
| `b` | one byte | `bool`, zero is false |
| `m` | low address byte, high address byte | internal RAM byte as `uint8` |
| `M` | low address byte, high address byte | internal RAM word as `uint16` |
| `a` | none | accumulator as `uint8` |
| `x` | none | X register as `uint8` |
| `y` | none | Y register as `uint8` |
| `p` | none | processor status flags as `uint8` |
| `s` | none | stack pointer as `uint8` |

Examples:

| Record bytes | Output |
|--------------|--------|
| `u`, `d`, `$C8` | `200` |
| `i`, `d`, `$C8` | `-56` |
| `0`, `4`, `U`, `x`, `$C2`, `$04` | `04c2` |
| `0`, `8`, `p`, `b` | the status flags as 8 binary digits |
| `b`, `t`, `$01` | `true` |

A RAM record reads internal RAM `$0000` to `$1FFF` and has no bus side effect.
A register record reads the CPU state at the time of the last record byte.
The instructions that write the record can change the registers and the flags.
To print the accumulator, write the record bytes with `LDX` and `STX`, not with
`LDA` and `STA`. A load instruction changes the N and Z flags, so a `p` record
does not show the flags from before the record.

## Errors

A record that cannot give a value prints a Go style error text:

| Text | Cause |
|------|-------|
| `%!z(BADTYPE)` | The type letter `z` is not known. |
| `%!(BADWIDTH)` | The flags, width, and precision have more than 8 characters. |
| `%!m(BADADDR)` | The RAM address is not in internal RAM. |
| `%!a(NOCPU)` | The register record has no CPU. |
| `%!(MISSING)` | A text write came before the end of the record. |

The Go `fmt` package prints an unsupported verb as usual, for example
`%!s(uint8=1)`. The emulator discards an incomplete record when the program
stops.

## Assembly Example

This example prints `score=` and one byte in decimal:

```asm
    lda #'s'
    sta $401B
    ; ... the other characters of "score="
    lda #'u'     ; type uint8
    sta $401C
    lda #'d'     ; verb
    sta $401C
    lda score    ; the value byte
    sta $401C
    lda #$0A     ; newline
    sta $401B
```

## RetroGo Programs

The RetroGo compiler changes `fmt.Print`, `fmt.Println`, `fmt.Printf`, `print`,
and `println` calls into writes to these ports when it compiles with
`-system nes`. The compiler formats the format string and all constant
arguments during compilation, and writes them to the text port. Only values
that the program calculates at run time go to the value port.

```go
fmt.Printf("hp=%d pos=%04x\n", hp, pos)
```

This call writes `hp=` and ` pos=` as text, one record for `hp`, one record for
`pos`, and a newline. The program does not format numbers at run time.
