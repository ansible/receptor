package types

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/workceptor"
	"github.com/grafana/pyroscope-go"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type NodeCfg struct {
	ID                               string                       `description:"Node ID. Defaults to the local hostname." barevalue:"yes"`
	DataDir                          string                       `description:"Directory in which to store node data." default:"/tmp/receptor"`
	FirewallRules                    []netceptor.FirewallRuleData `description:"Firewall rules, see documentation for syntax."`
	MaxIdleConnectionTimeout         string                       `description:"Maximum duration with no traffic before a backend connection is timed out and refreshed."`
	ReceptorKubeSupportReconnect     string
	ReceptorKubeClientsetQPS         string
	ReceptorKubeClientsetBurst       string
	ReceptorKubeClientsetRateLimiter string
}

var receptorDataDir string

func (cfg NodeCfg) Init() error {
	var err error
	if cfg.ID == "" {
		host, err := os.Hostname()
		if err != nil {
			return err
		}
		lchost := strings.ToLower(host)
		if lchost == "localhost" || strings.HasPrefix(lchost, "localhost.") {
			return fmt.Errorf("no node ID specified and local host name is localhost")
		}
		cfg.ID = host
	} else {
		submitIDRegex := regexp.MustCompile(`^[.\-_@:a-zA-Z0-9]*$`)
		match := submitIDRegex.FindSubmatch([]byte(cfg.ID))
		if match == nil {
			return fmt.Errorf("node id can only contain a-z, A-Z, 0-9 or special characters . - _ @ : but received: %s", cfg.ID)
		}
	}
	if strings.ToLower(cfg.ID) == "localhost" {
		return fmt.Errorf("node ID \"localhost\" is reserved")
	}

	receptorDataDir = cfg.DataDir

	netceptor.MainInstance = netceptor.New(context.Background(), cfg.ID)

	if len(cfg.FirewallRules) > 0 {
		rules, err := netceptor.ParseFirewallRules(cfg.FirewallRules)
		if err != nil {
			return err
		}
		err = netceptor.MainInstance.AddFirewallRules(rules, true)
		if err != nil {
			return err
		}
	}

	// update netceptor.MainInstance with the MaxIdleConnectionTimeout from the nodeCfg struct
	// this is a fall-forward mechanism. If the user didn't provide a value for MaxIdleConnectionTimeout in their configuration file,
	// we will apply the default timeout of 30s to netceptor.maxConnectionIdleTime
	if cfg.MaxIdleConnectionTimeout != "" {
		err = netceptor.MainInstance.SetMaxConnectionIdleTime(cfg.MaxIdleConnectionTimeout)
		if err != nil {
			return err
		}
	}

	workceptor.MainInstance, err = workceptor.New(context.Background(), netceptor.MainInstance, receptorDataDir)
	if err != nil {
		return err
	}
	controlsvc.MainInstance = controlsvc.New(true, netceptor.MainInstance)
	err = workceptor.MainInstance.RegisterWithControlService(controlsvc.MainInstance)
	if err != nil {
		return err
	}

	return nil
}

func (cfg NodeCfg) Run() error {
	workceptor.MainInstance.ListKnownUnitIDs() // Triggers a scan of unit dirs and restarts any that need it

	return nil
}

type ReceptorPyroscopeCfg struct {
	ApplicationName   string
	Tags              map[string]string
	ServerAddress     string // e.g http://pyroscope.services.internal:4040
	BasicAuthUser     string // http basic auth user
	BasicAuthPassword string // http basic auth password
	TenantID          string // specify TenantId when using phlare multi-tenancy
	UploadRate        string
	ProfileTypes      []string
	DisableGCRuns     bool // this will disable automatic runtime.GC runs between getting the heap profiles
	HTTPHeaders       map[string]string
}

type UploadRate struct {
	UploadRate time.Duration `yaml:"uploadRate"`
}

func (pyroscopeCfg ReceptorPyroscopeCfg) Init() error {
	if pyroscopeCfg.ApplicationName == "" {
		return nil
	}

	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	pyroscopeLogger := logrus.New()
	pyroscopeLogger.SetLevel(logrus.DebugLevel)

	if _, err := os.Stat(receptorDataDir); os.IsNotExist(err) {
		err := os.MkdirAll(receptorDataDir, 0o700)
		if err != nil {
			fmt.Printf("error creating directory: %v", err)
		}
	}

	logFile, err := os.OpenFile(fmt.Sprintf("%s/pyroscope.log", receptorDataDir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		pyroscopeLogger.Fatalf("Error opening log file: %v", err)
	}
	pyroscopeLogger.SetOutput(logFile)

	pyroscopeLogger.SetFormatter(&logrus.JSONFormatter{})

	_, err = pyroscope.Start(pyroscope.Config{
		ApplicationName:   pyroscopeCfg.ApplicationName,
		Tags:              pyroscopeCfg.Tags,
		ServerAddress:     pyroscopeCfg.ServerAddress,
		BasicAuthUser:     pyroscopeCfg.BasicAuthUser,
		BasicAuthPassword: pyroscopeCfg.BasicAuthPassword,
		TenantID:          pyroscopeCfg.TenantID,
		UploadRate:        getUploadRate(pyroscopeCfg),
		Logger:            pyroscopeLogger,
		ProfileTypes:      getProfileTypes(pyroscopeCfg),
		DisableGCRuns:     pyroscopeCfg.DisableGCRuns,
		HTTPHeaders:       pyroscopeCfg.HTTPHeaders,
	})

	if err != nil {
		return err
	} else {
		return nil
	}
}

func getUploadRate(cfg ReceptorPyroscopeCfg) time.Duration {
	if cfg.UploadRate == "" {
		return 15 * time.Second
	}
	var uploadRate UploadRate
	err := yaml.Unmarshal([]byte(cfg.UploadRate), &uploadRate)
	if err != nil {
		fmt.Println("failed to parse uploadRate from config file")
	}

	return uploadRate.UploadRate
}

func getProfileTypes(cfg ReceptorPyroscopeCfg) []pyroscope.ProfileType {
	profileType := []pyroscope.ProfileType{
		pyroscope.ProfileCPU,
		pyroscope.ProfileAllocObjects,
		pyroscope.ProfileAllocSpace,
		pyroscope.ProfileInuseObjects,
		pyroscope.ProfileInuseSpace,
	}
	if len(cfg.ProfileTypes) == 0 {
		return profileType
	}
	for _, pt := range cfg.ProfileTypes {
		switch pt {
		case "ProfileGoroutines":
			profileType = append(profileType, pyroscope.ProfileGoroutines)
		case "ProfileMutexCount":
			profileType = append(profileType, pyroscope.ProfileMutexCount)
		case "ProfileMutexDuration":
			profileType = append(profileType, pyroscope.ProfileMutexDuration)
		case "ProfileBlockCount":
			profileType = append(profileType, pyroscope.ProfileBlockCount)
		case "ProfileBlockDuration":
			profileType = append(profileType, pyroscope.ProfileBlockDuration)
		}
	}

	return profileType
}
