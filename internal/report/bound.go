package report

import "math"

// Upper is the one-sided upper bound of 95 % of a share counted over sheets (docs/architecture.md,
// 3.1, «Метрики»): the Clopper–Pearson bound on the effective size n / Deff, where Deff is the
// design effect of the sheets, 1 at least. events[i] of totals[i] are the sheet i's. With no events
// the bound is 1 − 0.05^(1/n_eff), about 3 / n_eff; with no totals at all it is 1.
func Upper(events, totals []int) float64 {
	n, x := 0, 0
	for i := range totals {
		n, x = n+totals[i], x+events[i]
	}
	if n == 0 {
		return 1
	}
	share := float64(x) / float64(n)
	eff := float64(n) / designEffect(events, totals, share, n)
	return clopperPearson(share*eff, eff)
}

// designEffect is how much the sheets widen the variance of a share against the same number of
// answers drawn apart: the variance of the ratio over the sheets, as clusters, over that of a
// binomial share, 1 at least. Sheets of one answer each have none, and neither has a share of 0
// or 1, or a single sheet.
func designEffect(events, totals []int, share float64, n int) float64 {
	k := 0
	sum := 0.0
	for i := range totals {
		if totals[i] == 0 {
			continue
		}
		k++
		d := float64(events[i]) - share*float64(totals[i])
		sum += d * d
	}
	if k < 2 || share == 0 || share == 1 {
		return 1
	}
	clusters := sum / (float64(n) * float64(n))
	binomial := share * (1 - share) / float64(n)
	return math.Max(1, clusters/binomial)
}

// clopperPearson is the one-sided upper bound of 95 % of x events in n, both of them sizes that may
// be fractions: the p at which the regularized incomplete beta I_p(x + 1, n − x) is 0.95, found by
// halving the interval.
func clopperPearson(x, n float64) float64 {
	if x >= n {
		return 1
	}
	lo, hi := x/n, 1.0
	for range 200 {
		mid := (lo + hi) / 2
		if incompleteBeta(mid, x+1, n-x) < 0.95 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi
}

// incompleteBeta is the regularized incomplete beta function I_p(a, b), by its continued fraction,
// taken on the side where it converges fast.
func incompleteBeta(p, a, b float64) float64 {
	switch {
	case p <= 0:
		return 0
	case p >= 1:
		return 1
	}
	la, _ := math.Lgamma(a + b)
	lb, _ := math.Lgamma(a)
	lc, _ := math.Lgamma(b)
	front := math.Exp(la - lb - lc + a*math.Log(p) + b*math.Log(1-p))
	if p < (a+1)/(a+b+2) {
		return front * betaFraction(p, a, b) / a
	}
	return 1 - front*betaFraction(1-p, b, a)/b
}

// betaFraction is the continued fraction of the incomplete beta, by Lentz's method.
func betaFraction(p, a, b float64) float64 {
	const tiny = 1e-300
	c, d := 1.0, 1-(a+b)*p/(a+1)
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1 / d
	h := d
	for m := 1; m <= 300; m++ {
		fm := float64(m)
		for _, num := range []float64{
			fm * (b - fm) * p / ((a + 2*fm - 1) * (a + 2*fm)),
			-(a + fm) * (a + b + fm) * p / ((a + 2*fm) * (a + 2*fm + 1)),
		} {
			d = 1 + num*d
			if math.Abs(d) < tiny {
				d = tiny
			}
			c = 1 + num/c
			if math.Abs(c) < tiny {
				c = tiny
			}
			d = 1 / d
			h *= d * c
		}
		if math.Abs(d*c-1) < 1e-15 {
			break
		}
	}
	return h
}
