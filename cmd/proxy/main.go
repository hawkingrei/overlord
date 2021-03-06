package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/felixhao/overlord/lib/log"
	"github.com/felixhao/overlord/proxy"
)

const (
	// VERSION version
	VERSION = "1.0.1"
)

var (
	version  bool
	logStd   bool
	logFile  string
	logVl    int
	debug    bool
	pprof    string
	config   string
	clusters clustersFlag
)

type clustersFlag []string

func (c *clustersFlag) String() string {
	return strings.Join([]string(*c), " ")
}

func (c *clustersFlag) Set(n string) error {
	*c = append(*c, n)
	return nil
}

var usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of Overlord proxy:\n")
	flag.PrintDefaults()
}

func init() {
	flag.Usage = usage
	flag.BoolVar(&version, "v", false, "print version.")
	flag.BoolVar(&logStd, "std", false, "log will printing into stdout.")
	flag.BoolVar(&debug, "debug", false, "debug model, will open stdout log. high priority than conf.debug.")
	flag.StringVar(&logFile, "log", "", "log will printing file {log}. high priority than conf.log.")
	flag.IntVar(&logVl, "log-vl", 0, "log verbose level. high priority than conf.log_vl.")
	flag.StringVar(&pprof, "pprof", "", "pprof listen addr. high priority than conf.pprof.")
	flag.StringVar(&config, "conf", "", "run with the specific configuration.")
	flag.Var(&clusters, "cluster", "specify cache cluster configuration.")
}

func main() {
	flag.Parse()
	if version {
		fmt.Printf("overlord version %s\n", VERSION)
		os.Exit(0)
	}
	c, ccs := parseConfig()
	if initLog(c) {
		defer log.Close()
	}
	// new proxy
	p, err := proxy.New(c)
	if err != nil {
		panic(err)
	}
	defer p.Close()
	go p.Serve(ccs)
	// hanlde signal
	signalHandler()
}

func initLog(c *proxy.Config) bool {
	var hs []log.Handler
	if logStd || c.Debug {
		hs = append(hs, log.NewStdHandler())
	}
	if c.Log != "" {
		hs = append(hs, log.NewFileHandler(c.Log))
	}
	if len(hs) > 0 {
		log.DefaultVerboseLevel = c.LogVL
		log.Init(hs...)
		return true
	}
	return false
}

func parseConfig() (c *proxy.Config, ccs []*proxy.ClusterConfig) {
	if config != "" {
		c = &proxy.Config{}
		if err := c.LoadFromFile(config); err != nil {
			panic(err)
		}
	} else {
		c = proxy.DefaultConfig()
	}
	// high priority start
	if pprof != "" {
		c.Pprof = pprof
	}
	if debug {
		c.Debug = debug
	}
	if logFile != "" {
		c.Log = logFile
	}
	if logVl > 0 {
		c.LogVL = logVl
	}
	// high priority end
	checks := map[string]struct{}{}
	for _, cluster := range clusters {
		cs := &proxy.ClusterConfigs{}
		if err := cs.LoadFromFile(cluster); err != nil {
			panic(err)
		}
		for _, cc := range cs.Clusters {
			if _, ok := checks[cc.Name]; ok {
				panic("the same cluster name cannot be repeated")
			}
			checks[cc.Name] = struct{}{}
		}
		ccs = append(ccs, cs.Clusters...)
	}
	return
}

func signalHandler() {
	var ch = make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		log.Infof("overlord proxy version[%s] already started", VERSION)
		si := <-ch
		log.Infof("overlord proxy version[%s] signal(%s) stop the process", VERSION, si.String())
		switch si {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			log.Infof("overlord proxy version[%s] already exited", VERSION)
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}
