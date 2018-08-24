package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-shellwords"

	"github.com/evo3cx/merror"
	"github.com/evo3cx/reenvoy"

	"github.com/hashicorp/consul-template/child"
	"github.com/hashicorp/consul-template/config"
	dep "github.com/hashicorp/consul-template/dependency"
	"github.com/hashicorp/consul-template/template"
	"github.com/hashicorp/consul-template/watch"
)

// Regexp for invalid characters in keys
var InvalidRegexp = regexp.MustCompile(`[^a-zA-Z0-9_]`)

type Runner struct {
	// ErrCh and DoneCh are channels where errors and finish notifacations occur.
	ErrCh  chan error
	DoneCh chan struct{}

	// ExitCh is a channel for parent processes to read exit status values from
	// the child processes
	ExitCh chan int

	// child is the child for process under management. this may be nil if not running
	// in exec Mode
	child *child.Child

	// childLock is the internal lock around the child process.
	childLock sync.RWMutex

	// config is the Config that created this Runner. it is used internally to construct
	// other obejcts and pass data.
	config *reenvoy.Config

	// configPrefixMap is a map of dependency's hashcode back to the config
	// prefix that created it.
	configPrefixMap map[string]*reenvoy.PrefixConfig

	// data is the latest representation of the data from Consul
	data map[string]interface{}

	// dependencies is the list of dependencies this runner is watching
	dependencies []dep.Dependency

	// dependenciesLock is a lock around touching the dependecies map
	dependenciesLock sync.Mutex

	// env is the last compiled environment
	env map[string]string

	// once indicates the runner should get data exactly one time and then stop.
	once bool

	// outSream and errStream are the io.Writer streams where the runner wil write information.
	outStream, errStream io.Writer

	// inStream is the ioReader where the runner will read information.
	inStream io.Reader

	// minTimer and maxTimer are used for not trouble or symptoms
	minTimer, maxTimer <-chan time.Time

	// stopLock is the lock around checking if the runner can be stopped
	stopLock sync.Mutex

	// stopped is a boolean of weather the runner is stopped
	stopped bool

	// watcher is the watcher this runner is using.
	watcher *watch.Watcher

	// brain is the internal storage database of returned dependency data.
	brain *template.Brain
}

func NewRunner(config *reenvoy.Config, once bool) (*Runner, error) {
	log.Printf("[INFO] (runner) creating new runner (once: %v)", once)

	runner := &Runner{
		config: config,
		once:   once,
	}

	if err := runner.init(); err != nil {
		return nil, err
	}

	return runner, nil
}

// Start creates a nenw runner and begins watching dependencies and quiescence
// timers. This is the main event loop and will block until finished
func (r *Runner) Start() {
	log.Println("[INFO] (runner) strating")

	// Add each dependency to the watcher
	for _, d := range r.dependencies {
		r.watcher.Add(d)
	}

	var exitCh <-chan int

	for {
		select {
		case data := <-r.watcher.DataCh():
			r.Receive(data.Dependency(), data.Data())

			// Drain all vies that have data
			// readmore about Break label in go https://www.ardanlabs.com/blog/2013/11/label-breaks-in-go.html
		OUTER:
			for {
				select {
				case data = <-r.watcher.DataCh():
					r.Receive(data.Dependency(), data.Data())
				default:
					break OUTER
				}
			}

			// if we are waiting for quiesecence, setup the timers
			if config.BoolVal(r.config.Wait.Enabled) {
				log.Printf("[INFO] (runner) quiescence timers starting")
				r.minTimer = time.After(config.TimeDurationVal(r.config.Wait.Min))
				if r.maxTimer == nil {
					r.maxTimer = time.After(config.TimeDurationVal(r.config.Wait.Max))
				}
				continue
			}

		case <-r.minTimer:
			log.Printf("[INFO] (runner) quiescence minTimer fired")
			r.minTimer, r.maxTimer = nil, nil
		case <-r.maxTimer:
			log.Printf("[INFO] (runner) quiescence maxTimer fired")
			r.minTimer, r.maxTimer = nil, nil
		case err := <-r.watcher.ErrCh():
			// Itentionally do not send the error back up to the runner. Eventually,
			// once consul API implements errwrap and multierror, we can check the "type"
			// of error and conditionally alert back.
			//
			// if err.Contains(Something){
			// 	errCh <- err
			// }
			log.Printf("[ERR] (runner) watcher reported error: %s", err)
			if r.once {
				r.ErrCh <- err
				return
			}
		case code := <-exitCh:
			r.ExitCh <- code
		case <-r.DoneCh:
			log.Printf("[INFO] (runner) received finish")
			return
		}

		// If we got this far, that means we got new data or one of the timers
		// fired, so attempt to re-process the environment.
		nexitCh, err := r.Run()
		if err != nil {
			r.ErrCh <- err
			return
		}

		// It's possible that we didn't start a process, in which case no exitCh
		// is returned. In this case, we should assume our current process is still
		// running and and move forward slowly. If we did get a new exitCh, that means a new
		// process is spawned, so we need to watch a new exitCh
		if nexitCh != nil {
			exitCh = nexitCh
		}

	}
}

func (r *Runner) Stop() {
	r.stopLock.Lock()
	defer r.stopLock.Unlock()

	if r.stopped {
		return
	}

	log.Println("[INFO] (runner) stopping")
	r.stopWatcher()
	r.stopChild()

	if err := r.deletePid(); err != nil {
		log.Println("[WARN] (runner) could not remove pid at %q: %s", r.config.PidFile, err)
	}

	r.stopped = true
	close(r.DoneCh)
}

func (r *Runner) stopWatcher() {
	if r.watcher != nil {
		log.Printf("[DEBUG] (runner) stopping watcher")
		r.watcher.Stop()
	}
}

func (r *Runner) Run() (<-chan int, error) {
	log.Printf("[INFO] (runner) running")

	env := make(map[string]string)

	// Iterate over each dependency and pull out its data. if any dependencies do not have
	// data yet, this function will immediately return because we cannot safely continue until all dependencies
	// have received data at least once.
	//
	// We iterate over the list of config prefix so that order is maintained.
	// since order in a map is not deteministic.
	r.dependenciesLock.Lock()
	defer r.dependenciesLock.Unlock()

	for _, d := range r.dependencies {
		data, ok := r.data[d.String()]
		if !ok {
			log.Printf("[INFO] (runner) missing data for %s", d)
			return nil, nil
		}

		switch typed := d.(type) {
		case *dep.KVListQuery:
			appendPrefixes(r, env, typed, data)
		case *dep.VaultReadQuery:
			appendSecrets(r, env, typed, data)
		default:
			return nil, fmt.Errorf("unknown dependency type %T", typed)
		}
	}

	// Print the final environment
	log.Printf("[TRACE] Environment:")
	for k, v := range env {
		log.Printf("[TRACE]   %s=%q", k, v)
	}

	// If the resulting map is the same, do not do anything. We use a length
	// check first to get a small performance increase if something has changed
	// so we don't immediately delegate to reflect which is slow
	if len(r.env) == len(env) && reflect.DeepEqual(r.env, env) {
		log.Printf("[INFO] (runner) environment was the same")
		return nil, nil
	}

	// Update the environment
	r.env = env

	if r.child != nil {
		log.Printf("[INFO] (runner) stopping existing child process")
		r.stopChild()
	}

	// create a new environment
	newEnv := make(map[string]string)

	// if we are not pristine, copy over all values in the current env.
	if !config.BoolVal(r.config.Pristine) {
		for _, v := range os.Environ() {
			list := strings.SplitN(v, "=", 2)
			newEnv[list[0]] = list[1]
		}
	}

	// Add our custom values, overwriting any existing one.
	for k, v := range r.env {
		newEnv[k] = v
	}

	// Prepare the final environment. Note that it's Crucial for us to
	// initialize this slice to an empty one vs. a nil one, since that's
	// how the child process class decides whether to pull in the parent's
	// environment or not, and we control that via pristine
	cmdEnv := make([]string, 0)
	for k, v := range newEnv {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}

	p := shellwords.NewParser()
	p.ParseEnv = true
	p.ParseBacktick = true
	args, err := p.Parse(config.StringVal(r.config.Exec.Command))
	if err != nil {
		return nil, merror.AppError(err, "failed parsing command")
	}

	child, err := child.New(&child.NewInput{
		Stdin:        r.inStream,
		Stdout:       r.outStream,
		Stderr:       r.errStream,
		Command:      args[0],
		Args:         args[1:],
		Env:          cmdEnv,
		Timeout:      0, // allow running indefinitely
		ReloadSignal: config.SignalVal(r.config.Exec.ReloadSignal),
		KillSignal:   config.SignalVal(r.config.Exec.KillSignal),
		KillTimeout:  config.TimeDurationVal(r.config.Exec.KillTimeout),
		Splay:        config.TimeDurationVal(r.config.Exec.Splay),
	})

	r.child = child

	return child.ExitCh(), err
}

// init creates the Runner's underlying data structures and returns an error if any problems accurs.
func (r *Runner) init() error {

	// Ensure default configuration values
	r.config = reenvoy.DefaultConfig()
	r.config.Finalize()

	// print the final config for debugging
	result, err := json.Marshal(r.config)
	if err != nil {
		return err
	}
	log.Printf("[DEBUG] (runner) final config: %s", result)

	// Create the clientset
	clients, err := newClientSet(r.config)
	if err != nil {
		return fmt.Errorf("runner: %s", err)
	}

	// create the watcher
	watcher, err := newWatcher(r.config, clients, r.once)
	if err != nil {
		return merror.AppError(err, "runner watcher error")
	}

	r.watcher = watcher
	r.data = make(map[string]interface{})
	r.configPrefixMap = make(map[string]*reenvoy.PrefixConfig)

	r.inStream = os.Stdin
	r.outStream = os.Stdout
	r.errStream = os.Stderr

	r.ErrCh = make(chan error)
	r.DoneCh = make(chan struct{})
	r.ExitCh = make(chan int, 1)

	// Parse and add consul dependencies
	for _, p := range *r.config.Prefixes {
		d, err := dep.NewKVListQuery(config.StringVal(p.Path))
		if err != nil {
			return err
		}

		r.dependencies = append(r.dependencies, d)
		r.configPrefixMap[d.String()] = p
	}

	// Parse and add vault dependencies - it is important that this come after consul
	// because consul should never be premitted to oeverwrite values from vault;
	// that would expose a security hole since access to consul is typically less controlled than acess to vault.
	for _, s := range *r.config.Secrets {
		path := config.StringVal(s.Path)
		log.Printf("looking at vault %s", path)
		d, err := dep.NewVaultReadQuery(path)
		if err != nil {
			return merror.AppError(err, "New Vault read query error")
		}
		r.dependencies = append(r.dependencies, d)
		r.configPrefixMap[d.String()] = s
	}

	return nil
}

// Receive accpets data from and map that data to the prefix
func (r *Runner) Receive(d dep.Dependency, data interface{}) {
	r.dependenciesLock.Lock()
	defer r.dependenciesLock.Unlock()

	log.Printf("[DEBUG] (runner) receiving dependency %s", d)
	r.data[d.String()] = data
}

func (r *Runner) stopChild() {
	r.childLock.RLock()
	defer r.childLock.RUnlock()

	if r.child != nil {
		log.Printf("[DEBUG] (runner) stopping child process")
		r.child.Stop()
	}
}

// deletePid is used to remove the PID on exit.
func (r *Runner) deletePid() error {
	path := config.StringVal(r.config.PidFile)
	if path == "" {
		return nil
	}

	log.Printf("[DEBUG] removing pid file at %q", path)

	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("runner: could not remove pid file: %s", err)
	}
	if stat.IsDir() {
		return fmt.Errorf("runner: specified pid file path is directory")
	}

	err = os.Remove(path)
	if err != nil {
		return fmt.Errorf("runner: could not remove pid file: %s", err)
	}
	return nil
}
