package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

const (
	name    = "tinyserver"
	version = "0.1.0dev"
)

type option struct {
	version   bool
	root      string
	allowAddr string
	port      string
	conf      string
	genConf   bool
}

var opt = &option{}

var confPath = func() string {
	confPath := "server.conf"
	if info, err := os.Stat(confPath); err == nil && info.Mode().IsRegular() {
		return confPath
	}
	u, err := user.Current()
	if err != nil {
		return confPath
	}
	if u.HomeDir != "" {
		return filepath.Join(u.HomeDir, confPath)
	}
	return confPath
}()

// AllowAddrMap is need readonly after init
var AllowAddrMap = make(map[string]bool)

func init() {
	sep := " "
	bind := "127.0.0.1:8080"
	log.SetPrefix("[" + name + " " + version + "]:")

	flag.BoolVar(&opt.version, "version", false, "show version")
	flag.StringVar(&opt.port, "port", bind, "listen address")
	flag.StringVar(&opt.root, "root", "", "specify serv root")
	flag.StringVar(&opt.allowAddr, "allow", "127.0.0.1", "allow address list. separator is space")
	// TODO: fix
	flag.StringVar(&opt.conf, "conf", confPath, "path to configuration file")
	flag.BoolVar(&opt.genConf, "gen-conf", false, "generate configuration file to os.Stdout")
	flag.Parse()

	if opt.conf != "" {
		if f, err := os.Open(opt.conf); err != nil {
			log.Println(err)
		} else {
			// trunc
			opt.allowAddr = ""
			defer f.Close()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				if sc.Err() != nil {
					log.Println(err)
					break
				}
				s := strings.TrimSpace(sc.Text())
				switch {
				case strings.HasPrefix(s, "#"):
					// pass comments
				case strings.HasPrefix(s, "allow="):
					opt.allowAddr = opt.allowAddr + sep + strings.TrimPrefix(s, "allow=")
				case strings.HasPrefix(s, "port="):
					if opt.port == bind {
						// TODO: warning message
					}
					opt.port = strings.TrimPrefix(s, "port=")
				case strings.HasPrefix(s, "root="):
					if opt.root == "" {
						// TODO: warning message
					}
					opt.root = strings.TrimPrefix(s, "root=")
				default:
					// TODO: error check
				}
			}
		}
	}

	for _, s := range strings.Split(opt.allowAddr, sep) {
		AllowAddrMap[s] = true
	}
}

// TODO: fix and impl ip6
func ipValidator(address string) string {
	isIP4 := func() bool {
		for _, r := range address {
			if r == '.' {
				return true
			}
		}
		return false
	}
	isValid := func(ip string) bool { return net.ParseIP(ip) == nil }
	switch {
	case isIP4():
		ip := address[:strings.IndexRune(address, ':')]
		if isValid(ip) {
			return ip
		}
		return ""
	default:
		// TODO: IP6
		if isValid(address) {
			return address
		}
		return ""
	}
}

// ServWithLog for with logger
func ServWithLog(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.RequestURI)

		switch {
		case AllowAddrMap[ipValidator(r.RemoteAddr)]:
			// pass
		default:
			log.Println("rejected ", r.RemoteAddr)
			http.Error(w, "Blocked", 403)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func genConf(w io.Writer) error {
	tmpl := `# generated by: ` + name + ` ` + version + `
# date: ` + fmt.Sprint(time.Now().Date()) + `
# for static file server

## list of allow remote IP address
#allow=127.0.0.1
#allow=192.168.1.x

## specify lesten port
#port=:8080
# or
# accept localhost only
#port=127.0.0.1:8080

## specify root directory
#root=public`
	_, err := fmt.Fprintln(w, tmpl)
	return err
}

func main() {
	if n := flag.NArg(); n == 1 {
		opt.root = flag.Arg(0)
	} else if n != 0 {
		log.Fatal("invalid argument:", flag.Args())
	}

	if opt.version {
		fmt.Printf("%s version %s\n", name, version)
		os.Exit(0)
	}

	if opt.root == "" {
		pwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		opt.root = pwd
	}

	if opt.genConf {
		if err := genConf(os.Stdout); err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if info, err := os.Stat(opt.root); err != nil {
		log.Fatal(err)
	} else if !info.IsDir() {
		log.Fatalf("is not directory: %v", opt.root)
	}

	msg := `[simple file server running]
options:
	opt.port: ` + opt.port + `
	opt.root: ` + opt.root + `
	opt.allowAddr: ` + opt.allowAddr + `
push ctrl-c then stopped`
	log.Println(msg)

	http.Handle("/", ServWithLog(http.FileServer(http.Dir(opt.root))))
	log.Fatal(http.ListenAndServe(opt.port, nil))
}
