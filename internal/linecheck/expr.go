package linecheck

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

// linear is a*u + b, where u is the unknown: what an expression of a linear equation comes to.
// Numbers are exact rationals, so 8/3 stays 8/3 and 0,1 + 0,2 is 0,3.
type linear struct {
	a, b *big.Rat
}

func constant(v *big.Rat) linear { return linear{a: new(big.Rat), b: v} }

func (l linear) isConstant() bool { return l.a.Sign() == 0 }

func (l linear) add(m linear) linear {
	return linear{a: new(big.Rat).Add(l.a, m.a), b: new(big.Rat).Add(l.b, m.b)}
}

func (l linear) neg() linear {
	return linear{a: new(big.Rat).Neg(l.a), b: new(big.Rat).Neg(l.b)}
}

func (l linear) scale(k *big.Rat) linear {
	return linear{a: new(big.Rat).Mul(l.a, k), b: new(big.Rat).Mul(l.b, k)}
}

// errNotChecked is what the parser returns for a line it will not judge: one it cannot read as
// the notation of the bank, or one outside what the measurement's tasks hold, such as a product
// of two unknowns. Such a line gets no verdict (R04 spec).
var errNotChecked = errors.New("not checked")

func notChecked(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errNotChecked, fmt.Sprintf(format, args...))
}

// parser reads one side of a line, already normalized, as a linear expression in the unknown.
type parser struct {
	s       []rune
	i       int
	unknown rune // 0: no unknown allowed
}

// parseExpr reads a whole expression and fails on anything left over.
func parseExpr(s string, unknown rune) (linear, error) {
	p := &parser{s: []rune(s), unknown: unknown}
	if len(p.s) == 0 {
		return linear{}, notChecked("an empty side")
	}
	l, err := p.sum()
	if err != nil {
		return linear{}, err
	}
	if p.i < len(p.s) {
		return linear{}, notChecked("%q is not understood", string(p.s[p.i:]))
	}
	return l, nil
}

func (p *parser) peek() rune {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

// sum := product (('+' | '-') product)*
func (p *parser) sum() (linear, error) {
	l, err := p.product()
	if err != nil {
		return linear{}, err
	}
	for {
		switch p.peek() {
		case '+', '-':
			op := p.peek()
			p.i++
			r, err := p.product()
			if err != nil {
				return linear{}, err
			}
			if op == '-' {
				r = r.neg()
			}
			l = l.add(r)
		default:
			return l, nil
		}
	}
}

// product := unary (('*' | ':' | '/') unary | primary)*, where a primary right after another
// factor multiplies it: 3x, 2(x + 1), (x + 1)(2).
func (p *parser) product() (linear, error) {
	l, err := p.unary()
	if err != nil {
		return linear{}, err
	}
	for {
		c := p.peek()
		switch {
		case c == '*' || c == ':' || c == '/':
			p.i++
			r, err := p.unary()
			if err != nil {
				return linear{}, err
			}
			if l, err = combine(l, r, c); err != nil {
				return linear{}, err
			}
		case startsPrimary(c):
			r, err := p.primary()
			if err != nil {
				return linear{}, err
			}
			if l, err = combine(l, r, '*'); err != nil {
				return linear{}, err
			}
		default:
			return l, nil
		}
	}
}

func startsPrimary(c rune) bool { return c == '(' || unicode.IsDigit(c) || unicode.IsLetter(c) }

// combine multiplies or divides two linear expressions, as long as the result is linear.
func combine(l, r linear, op rune) (linear, error) {
	if op == '*' {
		switch {
		case r.isConstant():
			return l.scale(r.b), nil
		case l.isConstant():
			return r.scale(l.b), nil
		default:
			return linear{}, notChecked("a product of two unknowns")
		}
	}
	if !r.isConstant() {
		return linear{}, notChecked("a division by the unknown")
	}
	if r.b.Sign() == 0 {
		return linear{}, notChecked("a division by zero")
	}
	return l.scale(new(big.Rat).Inv(r.b)), nil
}

// unary := ('-' | '+') unary | primary
func (p *parser) unary() (linear, error) {
	switch p.peek() {
	case '-':
		p.i++
		l, err := p.unary()
		if err != nil {
			return linear{}, err
		}
		return l.neg(), nil
	case '+':
		p.i++
		return p.unary()
	}
	return p.primary()
}

// primary := number | unknown | '(' sum ')'
func (p *parser) primary() (linear, error) {
	c := p.peek()
	switch {
	case c == '(':
		p.i++
		l, err := p.sum()
		if err != nil {
			return linear{}, err
		}
		if p.peek() != ')' {
			return linear{}, notChecked("a bracket is not closed")
		}
		p.i++
		return l, nil
	case unicode.IsDigit(c):
		return p.number()
	case unicode.IsLetter(c):
		p.i++
		if p.unknown == 0 || c != p.unknown {
			return linear{}, notChecked("the letter %q is not the unknown", c)
		}
		return linear{a: big.NewRat(1, 1), b: new(big.Rat)}, nil
	case c == 0:
		return linear{}, notChecked("the line ends where a number should be")
	default:
		return linear{}, notChecked("%q is not understood", c)
	}
}

// number := digits ((',' | '.') digits)?
func (p *parser) number() (linear, error) {
	start := p.i
	for unicode.IsDigit(p.peek()) {
		p.i++
	}
	if c := p.peek(); (c == ',' || c == '.') && p.i+1 < len(p.s) && unicode.IsDigit(p.s[p.i+1]) {
		p.i++
		for unicode.IsDigit(p.peek()) {
			p.i++
		}
	}
	text := strings.ReplaceAll(string(p.s[start:p.i]), ",", ".")
	v, ok := new(big.Rat).SetString(text)
	if !ok {
		return linear{}, notChecked("%q is not a number", text)
	}
	return constant(v), nil
}
