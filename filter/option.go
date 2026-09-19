package filter

// DefaultNegationPrefix is the query term prefix that marks a term as
// negative, excluding the lines it matches.
const DefaultNegationPrefix = "-"

// Option configures a filter at construction time.
type Option interface {
	apply(*filterOptions)
}

type filterOptions struct {
	negationPrefix string
}

type optionFunc func(*filterOptions)

func (f optionFunc) apply(o *filterOptions) { f(o) }

// WithNegationPrefix sets the query term prefix that marks a term as negative.
// An empty prefix turns negative matching off, and every term is then matched
// literally, including its leading hyphens.
func WithNegationPrefix(prefix string) Option {
	return optionFunc(func(o *filterOptions) {
		o.negationPrefix = prefix
	})
}

// buildOptions applies the given options on top of the defaults.
func buildOptions(options []Option) filterOptions {
	o := filterOptions{negationPrefix: DefaultNegationPrefix}
	for _, option := range options {
		option.apply(&o)
	}
	return o
}
