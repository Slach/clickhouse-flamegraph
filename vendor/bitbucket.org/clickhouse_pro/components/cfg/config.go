package cfg

import (
	"flag"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/clickhouse_pro/components/helpers"

	"github.com/jinzhu/configor"
	"github.com/kshvakov/clickhouse"
	"github.com/levigross/grequests"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/samuel/go-zookeeper/zk"
	"golang.org/x/oauth2/yandex"
	"gopkg.in/yaml.v2"
	"path"
)

//AppmetrikaURL baseURL for Yandex AppMetrika API
const AppmetrikaURL = "https://api.appmetrica.yandex.ru"

//MetrikaURL baseURL for Yandex Metrika API
const MetrikaURL = "https://api-metrika.yandex.ru"

//ConfigInterface Implementation of common configuration logic
type ConfigInterface interface {
	DefineCLIFlags()
	ParseCLIFlags()
	InitLoggers()
	LoadConfigFile(config interface{}, configFile, environment *string)
	MergeConfigFileWithCLIFlags()
	SaveConfigFile(config interface{}, configFile, environment *string)
}

//JSONIds just array of id
type JSONIds []struct {
	ID int `json:"id"`
}

//PartialJSONParsingStruct used in GetAvailableIDs
type PartialJSONParsingStruct struct {
	AppMetrikaIds JSONIds `json:"applications"`
	MetrikaIds    JSONIds `json:"counters"`
}

var appIDsJSON = PartialJSONParsingStruct{
	AppMetrikaIds: make(JSONIds, 0),
	MetrikaIds:    make(JSONIds, 0),
}

//ConfigType parent ConfigType for basically configuration functionality of CH.PRO Tools
type ConfigType struct {
	ConfigInterface
	Debug          bool
	Console        bool
	NonInteractive bool
	AutoSave       bool
	TmpDir         string
	DateSince      time.Time
	DateUntil      time.Time
	OAuth          struct {
		//nolint: golint
		ApplicationId     string
		ApplicationSecret string
		//@TODO 4h/DEV develop auto refresh tokens
		Token        string `json:"access_token" yaml:"access_token"`
		RefreshToken string `json:"refresh_token" yaml:"refresh_token"`
	}
	AutoLoadIds bool `yaml:"auto_load_ids"`
	Metrika     struct {
		CounterIds []string `yaml:"counter_ids"`
	}
	AppMetrika struct {
		ApplicationIds []string `yaml:"application_ids"`
	}
	Zookeeper struct {
		Hosts    []string
		Username string
		Password string
		Path     string
	}
	Clickhouse struct {
		Hosts          []string `yaml:"hosts"`
		TCPPort        string   `yaml:"tcp_port"`
		HTTPPort       string   `yaml:"http_port"`
		ClusterName    string
		Database       string
		Username       string
		Password       string
		TablePrefix    string
		TableSuffix    string
		BlockSize      uint64
		ReadTimeout    uint64 `yaml:"read_timeout"`
		WriteTimeout   uint64 `yaml:"write_timeout"`
		DropTable      bool
		UseDistributed bool
		UseReplicated  bool
	} `yaml:"clickhouse"`
}

//Environment suffix for config file
var Environment *string

//EnvPrefix prefix for Enviroment Variables
var EnvPrefix *string

//ConfigFile filename for loading configuration
var ConfigFile *string

var help *bool

var clickhouseHosts *string
var zookeeperHosts *string

var appmetrikaAppids *string
var metrikaAppids *string

var dateSince *string
var dateUntil *string

//InitConfig parse cli flags and config.yml and merge it in ConfigType inherited structure
//nolint: errcheck
func InitConfig(c ConfigInterface) {
	c.DefineCLIFlags()
	c.ParseCLIFlags()
	c.InitLoggers()
	c.LoadConfigFile(c, ConfigFile, Environment)
	c.MergeConfigFileWithCLIFlags()
	c.SaveConfigFile(c, ConfigFile, Environment)
}

//DefineCLIFlags can be inherited in child structures
func (c *ConfigType) DefineCLIFlags() {
	stdlog.SetOutput(log.Logger)

	Environment = flag.String("environment", "", "configuration Environment, if not empty try load config.yml and overwrite it by config.<Environment>.yml")
	EnvPrefix = flag.String("envPrefix", "CONF", "Environment variables prefix, if not empty try load <envPrefix>_CONFIG_VALUES")
	ConfigFile = flag.String("configFile", "", "path to configuration file i.e. config.yml, when use -enviroment=production settings , config.production.yml try load too")

	help = flag.Bool("help", false, "show usage")
	clickhouseHosts = flag.String("clickhouse.hosts", "", "comma separated Clickhouse Hosts with port 9000 used for Native protocol, i.e. 127.0.0.1:9000")
	zookeeperHosts = flag.String("zookeeper.hosts", "", "comma separated Zookeeper Hosts with port used for Zookeeper for grab clickhouse hosts list, i.e. 127.0.0.1:2181")

	metrikaAppids = flag.String("metrika.app_ids", "", "comma separated Yandex Metrika Counter IDs, you cat get it over https://metrika.yandex.ru/list")
	appmetrikaAppids = flag.String("appmetrika.app_ids", "", "comma separated Yandex AppMetrica Counter IDs, you cat get it over https://appmetrica.yandex.ru/application/list")

	flag.BoolVar(&c.AutoLoadIds, "auto_load_ids", true, "if -metrika.app_ids or -appmetrika.app_ids empty try auto load IDs from API")

	dateSince = flag.String("date.since", "", "use time period since in YYYY-MM-DD HH:II:SS format")
	dateUntil = flag.String("date.until", "", "use time period until in YYYY-MM-DD HH:II:SS format")

	flag.BoolVar(&c.Debug, "debug", false, "show debug log")
	flag.BoolVar(&c.Console, "console", false, "use simple console logging without JSON structured log")
	flag.BoolVar(&c.NonInteractive, "non-interactive", false, "don't use interactive prompts")
	flag.BoolVar(&c.AutoSave, "save-config", false, "auto save parameters to <ConfigFile>")

	flag.StringVar(&c.TmpDir, "tmpdir", "/tmp/", "directory for save temporary files")

	flag.StringVar(&c.Zookeeper.Username, "zookeeper.username", "", "Digest authorization user in Zookeeper")
	flag.StringVar(&c.Zookeeper.Password, "zookeeper.password", "", "Digest authorization password in Zookeeper")
	flag.StringVar(&c.Zookeeper.Path, "zookeeper.path", "/clickhouse", "Path for Clickhouse nodes in Zookeeper")

	flag.StringVar(&c.Clickhouse.ClusterName, "clickhouse.clustername", "default", "Clickhouse cluster name used for distributed table")
	flag.StringVar(&c.Clickhouse.Database, "clickhouse.database", "default", "Clickhouse database name")
	flag.StringVar(&c.Clickhouse.Username, "clickhouse.username", "default", "Clickhouse username")
	flag.StringVar(&c.Clickhouse.Password, "clickhouse.password", "", "Clickhouse password")
	flag.StringVar(&c.Clickhouse.TablePrefix, "clickhouse.tableprefix", "", "Clickhouse prefix for tables names")
	flag.StringVar(&c.Clickhouse.HTTPPort, "clickhouse.http_port", "8123", "clickhouse port for HTTP connection")
	flag.StringVar(&c.Clickhouse.TCPPort, "clickhouse.tcp_port", "9000", "clickhouse port for TCP connection")
	flag.BoolVar(&c.Clickhouse.DropTable, "clickhouse.droptable", false, "Drop tables before create it")
	flag.Uint64Var(&c.Clickhouse.BlockSize, "clickhouse.blocksize", 100000, "maximum rows in block (default is 100000). If the rows are larger then the data will be split into several blocks to send them to Clickhouse server")

	flag.Uint64Var(&c.Clickhouse.ReadTimeout, "clickhouse.read_timeout", 600, "read timeout to all connections to Clickhouse servers")
	flag.Uint64Var(&c.Clickhouse.WriteTimeout, "clickhouse.write_timeout", 600, "write timeout to all connections to Clickhouse servers")

	flag.StringVar(&c.OAuth.ApplicationId, "oauth.application_id", "", "Yandex oAuth application id get it from https://oauth.yandex.ru/ or create new application (https://oauth.yandex.ru/client/new) if you need")
	flag.StringVar(&c.OAuth.ApplicationSecret, "oauth.application_secret", "", "Yandex oAuth application secret (password) get it from https://oauth.yandex.ru/ or create new application (https://oauth.yandex.ru/client/new) if you need")
	flag.StringVar(&c.OAuth.Token, "oauth.token", "", fmt.Sprintf("Yandex oAuth token get it from %s you need use callback url https://oauth.yandex.ru/verification_code when create oauth application", yandex.Endpoint.TokenURL))

}

//ParseCLIFlags may be override behavior in inherited structure
func (c *ConfigType) ParseCLIFlags() {
	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(1)
	}
}

//InitLoggers init clickhouse, zerolog and std loggers, may be override behavior in inherited structure
func (c *ConfigType) InitLoggers() {
	if c.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	if c.Console {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
	clickhouse.SetLogOutput(log.Logger)
	zk.DefaultLogger = &log.Logger
}

//MergeConfigFileWithCLIFlags parse some complex fields from CLI flags and merge them with existing ConfigType structure structure
func (c *ConfigType) MergeConfigFileWithCLIFlags() {
	c.ParseZookeeperHosts()
	c.ParseClickhouseHosts()

	if len(c.Zookeeper.Hosts) == 0 && len(c.Clickhouse.Hosts) == 0 {
		flag.Usage()
		log.Fatal().
			Strs("Clickhouse.Hosts", c.Clickhouse.Hosts).
			Strs("Zookeeper.Hosts", c.Zookeeper.Hosts).
			Msg("need ZooKeeper or ClickHouse hosts")
	}
	for i, host := range c.Clickhouse.Hosts {
		if !strings.Contains(host, ":") {
			c.Clickhouse.Hosts[i] += ":" + c.Clickhouse.TCPPort
		}
	}

	if c.OAuth.ApplicationId == "" {
		c.ShowErrorMessageWhenNonInteractive("Missed oauth.application_id for non-interactive mode")
		c.OAuth.ApplicationId = helpers.ReadInputConsoleString("Open https://oauth.yandex.ru/ and view list of your oauth application\n or create new one on https://oauth.yandex.ru/client/new\n you need use callback url https://oauth.yandex.ru/verification_code when create oauth application\n copy ID field and paste here:\n")
	}
	if c.OAuth.ApplicationSecret == "" {
		c.ShowErrorMessageWhenNonInteractive("Missed oauth.application_secret for non-interactive mode")
		c.OAuth.ApplicationSecret = helpers.ReadInputConsoleString("Open https://oauth.yandex.ru/ and view list of your oauth application\n or create new one on https://oauth.yandex.ru/client/new\n you need use callback url https://oauth.yandex.ru/verification_code when create oauth application\n copy Secret (Password) field and paste here:\n")
	}

	if c.OAuth.Token == "" {
		c.ParseOAuthToken()
	}
	c.ParseDatePeriod(dateSince, dateUntil)
	c.ParseAppIDs(&c.Metrika.CounterIds, metrikaAppids)
	c.ParseAppIDs(&c.AppMetrika.ApplicationIds, appmetrikaAppids)
	if c.AutoLoadIds {
		c.GetAvailableIDs(&c.Metrika.CounterIds, &appIDsJSON, MetrikaURL+"/management/v1/counters", "Metrika")
		c.GetAvailableIDs(&c.AppMetrika.ApplicationIds, &appIDsJSON, AppmetrikaURL+"/management/v1/applications", "AppMetrika")
	}
}

//ParseZookeeperHosts split CLI flags by comma, if current hosts empty
func (c *ConfigType) ParseZookeeperHosts() {
	if (c.Zookeeper.Hosts == nil || len(c.Zookeeper.Hosts) == 0) && *zookeeperHosts != "" {
		c.Zookeeper.Hosts = strings.Split(*zookeeperHosts, ",")
	}
}

//ParseClickhouseHosts split CLI flags by comma, if current hosts empty
func (c *ConfigType) ParseClickhouseHosts() {
	if (c.Clickhouse.Hosts == nil || len(c.Clickhouse.Hosts) == 0) && *clickhouseHosts != "" {
		c.Clickhouse.Hosts = strings.Split(*clickhouseHosts, ",")
	}
}

//LoadConfigFile unmarshal configFile into config structure
// configFile is full path
// if environment!="" config structure will be load from configFile.<environment>.ext
func (c *ConfigType) LoadConfigFile(config interface{}, configFile *string, environment *string) {
	if _, err := os.Stat(*ConfigFile); err == nil {
		err = configor.New(&configor.Config{Environment: *Environment, ENVPrefix: *EnvPrefix, Debug: c.Debug, Verbose: c.Debug}).Load(config, *ConfigFile)
		if err != nil {
			log.Fatal().Err(err).Str("ConfigFile", *ConfigFile).Msg("Error in configuration file")
		}
	} else if *ConfigFile != "" {
		cwd, cwdErr := os.Getwd()
		log.Error().Str("currentDir", cwd).Str("ConfigFile", *ConfigFile).Err(err).Err(cwdErr).Msg("error in LoadConfigFile")
	}
}

//SaveConfigFile marshal structure into configFile
// configFile is full path
// if environment!="" config structure will be save to configFile.<environment>.ext
func (c *ConfigType) SaveConfigFile(config interface{}, configFile *string, environment *string) {
	if c.AutoSave {
		yamlName := *configFile
		if *environment != "" {
			ext := path.Ext(*configFile)
			yamlName = yamlName[0:len(yamlName)-len(ext)] + "." + *environment + ext
		}
		cfg, err := yaml.Marshal(config)
		if err != nil {
			log.Fatal().Str("ConfigFile", yamlName).Err(err).Msg("Error serialize Config to YAML")
		}
		err = ioutil.WriteFile(yamlName, cfg, 0644)
		if err != nil {
			log.Fatal().Str("ConfigFile", yamlName).Err(err).Msg("Error save Config to YAML")
		}
		log.Debug().Str("yamlName", yamlName).Msgf("save config=%s", cfg)
	}
}

//ParseDatePeriod parse date in YYYY-MM-DD HH:MI:SS format
func (c *ConfigType) ParseDatePeriod(dateSince *string, dateUntil *string) {
	var err error
	if c.DateSince.Year() == 1 {
		if *dateSince != "" {
			if c.DateSince, err = time.Parse("2006-01-02 15:04:05", *dateSince); err != nil {
				c.DateSince = helpers.BeginOfDay(time.Now().UTC().AddDate(0, -1, 0))
			}
		} else {
			c.DateSince = helpers.BeginOfDay(time.Now().UTC().AddDate(0, -1, 0))
		}
	}
	if c.DateUntil.Year() == 1 {
		if *dateUntil != "" {
			if c.DateUntil, err = time.Parse("2006-01-02 15:04:05", *dateUntil); err != nil {
				c.DateUntil = helpers.EndOfDay(time.Now().UTC().AddDate(0, 0, -1))
			}
		} else {
			c.DateUntil = helpers.EndOfDay(time.Now().UTC().AddDate(0, 0, -1))
		}
	}

}

//ParseOAuthToken interactive read oauth token from stdin
func (c *ConfigType) ParseOAuthToken() {
	c.ShowErrorMessageWhenNonInteractive("Missed oauth.token for non-interactive mode")
	code := helpers.ReadInputConsoleString("Open in your browser following URL\n %s?response_type=code&client_id=%s&scope=appmetrica:read+metrika:read\nand enter given CODE here:\n", yandex.Endpoint.AuthURL, c.OAuth.ApplicationId)
	fmt.Printf("Get OAuth token from code %s\n", code)
	resp, err := grequests.Post(
		yandex.Endpoint.TokenURL,
		&grequests.RequestOptions{Data: map[string]string{
			"grant_type":    "authorization_code",
			"code":          code,
			"client_id":     c.OAuth.ApplicationId,
			"client_secret": c.OAuth.ApplicationSecret,
		}},
	)
	if err != nil || !resp.Ok {
		log.Fatal().Int("response_status", resp.StatusCode).Str("response_body", resp.String()).Err(err).Msg("OAuth Response error")
	}
	if err = resp.JSON(&c.OAuth); err != nil {
		log.Fatal().Int("response_status", resp.StatusCode).Str("response_body", resp.String()).Err(err).Msg("OAuth JSON Parsing error")
	}
}

//ParseAppIDs append only new AppIds into selected []string slice if values not exists
func (c *ConfigType) ParseAppIDs(listIDs *[]string, newAppIds *string) {
	if *newAppIds != "" {
		for _, id := range strings.Split(*newAppIds, ",") {
			found := false
			for _, existsID := range *listIDs {
				if id == existsID {
					found = true
				}
			}
			if !found {
				*listIDs = append(*listIDs, id)
			}
		}
		return
	}
}

//GetAvailableIDs append available IDs over API into selected []string slice
func (c *ConfigType) GetAvailableIDs(listIDs *[]string, tmpJSON *PartialJSONParsingStruct, apiURL string, appIdsName string) {
	log.Info().Msgf("Get %s", apiURL)
	resp, err := grequests.Get(
		apiURL,
		&grequests.RequestOptions{
			Params: map[string]string{
				"oauth_token": c.OAuth.Token,
			},
		},
	)
	errLog := log.With().Int("response_status", resp.StatusCode).Str("response_body", resp.String()).Err(err).Logger()
	if err != nil || !resp.Ok {
		errLog.Error().Msg("GetAvailableIDs Response error")
		return
	}
	if err = resp.JSON(tmpJSON); err != nil {
		errLog.Error().Msg("GetAvailableIDs JSON Parsing error")
		return
	}
	var ids JSONIds
	switch appIdsName {
	case "AppMetrika":
		ids = tmpJSON.AppMetrikaIds
	default:
		ids = tmpJSON.MetrikaIds
	}
	for _, id := range ids {
		found := false
		newID := strconv.Itoa(id.ID)
		for _, existsID := range *listIDs {
			if newID == existsID {
				found = true
			}
		}
		if !found {
			*listIDs = append(*listIDs, strconv.Itoa(id.ID))
		}
	}
	log.Info().Strs("listIDs", *listIDs).Msg("listIDs filled")
}

//ShowErrorMessageWhenNonInteractive Show message and usage and exit after that
func (c *ConfigType) ShowErrorMessageWhenNonInteractive(msg string) {
	if c.NonInteractive {
		log.Info().Msg(msg)
		flag.Usage()
		os.Exit(1)
	}
}
