// MIT License

//
// Copyright (c) 2019 kpango (Yusuke Kato)
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package tcp provides tcp option
package tcp

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/kpango/gache"
	"github.com/vdaas/vald/internal/log"
	"github.com/vdaas/vald/internal/safety"
)

type Dialer func(ctx context.Context, network, host string) (net.Conn, error)

type dialer struct {
	cache              gache.Gache
	dnsCache           bool
	dnsRefreshDuration time.Duration
	dnsCacheExpiration time.Duration
	dialerTimeout      time.Duration
	dialerKeepAlive    time.Duration
	dialerDualStack    bool
	der                *net.Dialer
}

func NewDialer(ctx context.Context, opts ...DialerOption) Dialer {

	d := new(dialer)
	for _, opt := range append(defaultDialerOptions, opts...) {
		opt(d)
	}

	d.der = &net.Dialer{
		Timeout:   d.dialerTimeout,
		KeepAlive: d.dialerKeepAlive,
		DualStack: d.dialerDualStack,
		Control:   Control,
	}

	d.der.Resolver = &net.Resolver{
		PreferGo: false,
		Dial:     d.der.DialContext,
	}

	if !d.dnsCache || d.cache == nil {
		return d.der.DialContext
	}

	if d.dnsRefreshDuration > d.dnsCacheExpiration {
		d.dnsRefreshDuration, d.dnsCacheExpiration =
			d.dnsCacheExpiration, d.dnsRefreshDuration
	}

	d.startDialerCache(ctx)

	return d.cachedDialer
}

func (d *dialer) lookup(ctx context.Context, host string) (ips map[int]string, err error) {
	cache, ok := d.cache.Get(host)
	if ok {
		return cache.(map[int]string), nil
	}

	r, err := d.der.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	ips = make(map[int]string, len(r))
	for i, ip := range r {
		ips[i] = ip.String()
	}

	d.cache.SetWithExpire(host, ips,
		d.dnsCacheExpiration)

	return ips, nil
}

func (d *dialer) startDialerCache(ctx context.Context) {
	d.cache.SetDefaultExpire(d.dnsCacheExpiration).
		SetExpiredHook(func(gctx context.Context, host string) {
			if err := safety.RecoverFunc(func() (err error) {
				_, err = d.lookup(gctx, host)
				return err
			}); err != nil {
				log.Error(err)
			}
		}).
		EnableExpiredHook().
		StartExpired(ctx, d.dnsRefreshDuration)
}

func (d *dialer) cachedDialer(dctx context.Context, network, host string) (
	conn net.Conn, err error) {

	sep := strings.LastIndex(host, ":")

	ips, err := d.lookup(dctx, host[:sep])
	if err == nil {
		for _, ip := range ips {
			conn, err = d.der.DialContext(dctx, network,
				ip+host[sep:])
			if err == nil {
				return conn, nil
			}
			if conn != nil {
				conn.Close()
			}
		}
		d.cache.Delete(host[:sep])
	}

	return d.der.DialContext(dctx, network, host)
}
