package hsdp

import "github.com/philips-software/go-hsdp-api/console"

func WithClient(client *console.Client) OptionFunc {
	return func(m *Metric) error {
		m.client = client
		return nil
	}
}

func WithName(name string) OptionFunc {
	return func(m *Metric) error {
		m.name = name
		return nil
	}
}

func WithHelp(help string) OptionFunc {
	return func(m *Metric) error {
		m.help = help
		return nil
	}
}

func WithQuery(query string) OptionFunc {
	return func(m *Metric) error {
		m.query = query
		return nil
	}
}

func WithService(service string) OptionFunc {
	return func(m *Metric) error {
		m.service = service
		return nil
	}
}
