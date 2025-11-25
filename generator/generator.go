package generator

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unsafe"
)

const (
	addressOffset = 8000000 // 8 MB

	inR  uint8 = 0b10000000
	inS2 uint8 = 0b00100000
	inS3 uint8 = 0b01000000
	inS4 uint8 = 0b01100000
)

const (
	inLit uint8 = iota
	inDei
	inDeo
	inHlt
	inBrp
	inDup
	inSwp
	inRot
	inPop
	inSth
	inGpc
	inMemr
	inMemw
	inMemc
	inJmp
	inTjmp
	inFjmp
	inEq
	inNe
	inLt
	inGt
	inAdd
	inSub
	inMul
	inDiv
	inAnd
	inOr
	inXor
	inNot
)

func getIns(name string) (ins uint8, usesR, usesS, ok bool) {
	ok = true
	usesR = true
	usesS = true

	switch name {
	case "LIT":
		ins = inLit
	case "DEI":
		ins = inDei
	case "DEO":
		ins = inDeo
	case "HLT":
		ins = inHlt
	case "BRP":
		ins = inBrp
		usesR = false
		usesS = false
	case "DUP":
		ins = inDup
	case "SWP":
		ins = inSwp
	case "ROT":
		ins = inRot
	case "POP":
		ins = inPop
	case "STH":
		ins = inSth
	case "GPC":
		ins = inGpc
		usesS = false
	case "MEMR":
		ins = inMemr
	case "MEMW":
		ins = inMemw
	case "MEMC":
		ins = inMemc
	case "JMP":
		ins = inJmp
		usesS = false
	case "TJMP":
		ins = inTjmp
	case "FJMP":
		ins = inFjmp
	case "EQ":
		ins = inEq
	case "NE":
		ins = inNe
	case "LT":
		ins = inLt
	case "GT":
		ins = inGt
	case "ADD":
		ins = inAdd
	case "SUB":
		ins = inSub
	case "MUL":
		ins = inMul
	case "DIV":
		ins = inDiv
	case "AND":
		ins = inAnd
	case "OR":
		ins = inOr
	case "XOR":
		ins = inXor
	case "NOT":
		ins = inNot
	default:
		ok = false
	}

	return
}

func isSymbol(ch byte) bool {
	return ch > 32 && ch < 127 && (ch < '0' || ch > '9') && ch != ';' && ch != '@' && ch != '%' && ch != '!' && ch != '(' && ch != ')'
}

func isHex(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')
}

type Generator struct {
	r         io.Reader
	err       error
	labels    map[string]int
	resols    map[string][]int
	file      string
	buf       []byte
	ln, col   int
	cur, next byte
	eof       bool
}

func New(file string, r io.Reader) Generator {
	g := Generator{
		r:      r,
		labels: map[string]int{},
		resols: map[string][]int{},
		file:   file,
		ln:     1,
	}

	g.byte()
	g.byte()

	g.col = 1

	return g
}

func (g *Generator) Errfp(ln, col int, msg string, a ...any) error {
	return fmt.Errorf("%s:%d:%d: %s", g.file, ln, col, fmt.Sprintf(msg, a...))
}

func (g *Generator) Errf(msg string, a ...any) error {
	return g.Errfp(g.ln, g.col, msg, a...)
}

func (g *Generator) end() bool {
	return g.err != nil || g.eof
}

func (g *Generator) byte() {
	g.cur = g.next

	g.col++

	if g.cur == '\n' {
		g.ln++
		g.col = 1
	}

	_, g.err = g.r.Read(unsafe.Slice(&g.next, 1))

	if g.err == io.EOF {
		g.err = nil
		g.eof = true
	}
}

func (g *Generator) collectHex(ln, col int) int {
	i := 0

	fromHex := func(hc byte) byte {
		if hc >= '0' && hc <= '9' {
			hc -= '0'
		} else if hc >= 'a' && hc <= 'f' {
			hc -= 'a'
			hc += 10
		}

		return hc
	}

	for i < 4 && isHex(g.cur) {
		g.buf = append(g.buf, fromHex(g.cur))

		g.byte()

		if g.end() || !isHex(g.cur) {
			g.err = g.Errfp(ln, col, "Invalid number")

			return 0
		}

		g.buf[len(g.buf)-1] <<= 4
		g.buf[len(g.buf)-1] |= fromHex(g.cur)

		g.byte()

		i++
	}

	if isHex(g.cur) {
		g.err = g.Errfp(ln, col, "Invalid number")
	}

	return i
}

func (g *Generator) Generate() (rbuf []byte, rerr error) {
	defer func() {
		rerr = g.err

		if rerr == nil {
			rbuf = g.buf
		}
	}()

	for {
		switch g.cur {
		case '(':
			{
				ln, col := g.ln, g.col

				for !g.end() && g.cur != ')' {
					g.byte()
				}

				if g.cur != ')' {
					g.err = g.Errfp(ln, col, "Unterminated comment")
				}

				g.byte()
			}
		case ';':
			{
				ln, col := g.ln, g.col

				sb := strings.Builder{}

				g.byte()

				for !g.end() && isSymbol(g.cur) {
					sb.WriteByte(g.cur)
					g.byte()
				}

				if sb.Len() == 0 {
					g.err = g.Errfp(ln, col, "Label literals cannot be empty")
				}

				sym := sb.String()

				g.buf = append(g.buf, inLit|inS4, 0, 0, 0, 0)
				g.resols[sym] = append(g.resols[sym], len(g.buf)-4)
			}
		case '@':
			{
				ln, col := g.ln, g.col

				sb := strings.Builder{}

				g.byte()

				for !g.end() && isSymbol(g.cur) {
					sb.WriteByte(g.cur)
					g.byte()
				}

				if sb.Len() == 0 {
					g.err = g.Errfp(ln, col, "Label definitions cannot be empty")
					return
				}

				sym := sb.String()

				if _, ok := g.labels[sym]; ok {
					g.err = g.Errfp(ln, col, "Cannot redefine label '%s'", sym)
					return
				}

				g.labels[sym] = len(g.buf)
			}
		case '\'':
			{
				ln, col := g.ln, g.col

				g.byte()

				n := 0

				for !g.end() && !unicode.IsSpace(rune(g.cur)) {
					g.buf = append(g.buf, g.cur)
					n++
					g.byte()
				}

				if n == 0 {

					g.err = g.Errfp(ln, col, "String literals cannot empty")
					return
				}
			}
		case '#':
			{
				ln, col := g.ln, g.col

				g.byte()

				g.buf = append(g.buf, inLit)

				ins := &g.buf[len(g.buf)-1]

				n := g.collectHex(ln, col)

				switch n {
				case 2:
					*ins |= inS2
				case 3:
					*ins |= inS3
				case 4:
					*ins |= inS4
				default:
					*ins &= 0b10011111
				}
			}
		default:
			if unicode.IsSpace(rune(g.cur)) {
				g.byte()
			} else if isHex(g.cur) {
				g.collectHex(g.ln, g.col)
			} else if isSymbol(g.cur) {
				ln, col := g.ln, g.col

				r := false
				var s uint8
				sb := strings.Builder{}

				for !g.end() && isSymbol(g.cur) {
					sb.WriteByte(g.cur)
					g.byte()
				}

				if !g.end() && g.cur == 'r' {
					r = true
					g.byte()
				}

				if !g.end() && g.cur >= '0' && g.cur <= '9' {
					adv := true

					switch g.cur {
					case '2':
						s = inS2
					case '3':
						s = inS3
					case '4':
						s = inS4
					default:
						adv = false
					}

					if adv {
						g.byte()
					}
				}

				name := sb.String()

				if !g.end() && !unicode.IsSpace(rune(g.cur)) {
					sb.WriteByte(g.cur)
				}

				ins, usesR, usesS, ok := getIns(name)
				if !ok {
					g.err = g.Errfp(ln, col, "Unknown symbol '%s'", name)
					return
				} else if !usesR && r {
					g.err = g.Errfp(ln, col, "Instruction '%s' is incompatible with the return modifier", name)
					return
				} else if !usesS && s != 0 {
					g.err = g.Errfp(ln, col, "Instruction '%s' is incompatible with the size modifier", name)
					return
				}

				if r {
					ins |= inR
				}

				g.buf = append(g.buf, ins|s)
			} else {
				g.err = g.Errf("Unexpected character '%c' (%d)", g.cur, g.cur)
				return
			}
		}

		if g.end() {
			break
		}
	}

	for sym, pos := range g.labels {
		if res, ok := g.resols[sym]; ok {
			for _, idx := range res {
				binary.BigEndian.PutUint32(unsafe.Slice(&g.buf[idx], 4), uint32(addressOffset-len(g.buf)+pos))
			}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: label '%s' is unused\n", sym)
		}
	}

	return
}
