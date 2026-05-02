package health

import (
	"context"
	"time"
)

type Poller struct {
	Checks   []Checker
	Interval time.Duration
	Timeout  time.Duration
}

func NewPoller(checks []Checker, interval, timeout time.Duration) *Poller {
	return &Poller{
		Checks:   checks,
		Interval: interval,
		Timeout:  timeout,
	}
}

func (p *Poller) PollAll(ctx context.Context) <-chan Status {
	ch := make(chan Status, len(p.Checks))
	go func() {
		defer close(ch)
		deadline := time.After(p.Timeout)
		healthy := make(map[string]bool)

		for {
			allHealthy := true
			for _, check := range p.Checks {
				if healthy[check.Name()] {
					continue
				}
				allHealthy = false

				start := time.Now()
				err := check.Check(ctx)
				elapsed := time.Since(start)

				if err == nil {
					healthy[check.Name()] = true
					ch <- Status{Name: check.Name(), Healthy: true, Elapsed: elapsed}
				} else {
					ch <- Status{Name: check.Name(), Healthy: false, Err: err, Elapsed: elapsed}
				}
			}

			if allHealthy {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-deadline:
				for _, check := range p.Checks {
					if !healthy[check.Name()] {
						ch <- Status{
							Name: check.Name(),
							Err:  context.DeadlineExceeded,
						}
					}
				}
				return
			case <-time.After(p.Interval):
			}
		}
	}()
	return ch
}
