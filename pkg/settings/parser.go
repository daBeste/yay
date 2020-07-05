package settings

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/leonelquinteros/gotext"
	rpc "github.com/mikkeloscar/aur"
	"github.com/pkg/errors"
)

type Option struct {
	Args []string
}

func (o *Option) Add(arg string) {
	if o.Args == nil {
		o.Args = []string{arg}
		return
	}
	o.Args = append(o.Args, arg)
}

func (o *Option) First() string {
	if o.Args == nil || len(o.Args) == 0 {
		return ""
	}
	return o.Args[0]
}

func (o *Option) Set(arg string) {
	o.Args = []string{arg}
}

// Arguments Parses command line arguments in a way we can interact with programmatically but
// also in a way that can easily be passed to pacman later on.
type Arguments struct {
	Op      string
	Options map[string]*Option
	Globals map[string]*Option
	Targets []string
}

func MakeArguments() *Arguments {
	return &Arguments{
		"",
		make(map[string]*Option),
		make(map[string]*Option),
		make([]string, 0),
	}
}

func (parser *Arguments) CopyGlobal() *Arguments {
	cp := MakeArguments()
	for k, v := range parser.Globals {
		cp.Globals[k] = v
	}

	return cp
}

func (parser *Arguments) Copy() (cp *Arguments) {
	cp = MakeArguments()

	cp.Op = parser.Op

	for k, v := range parser.Options {
		cp.Options[k] = v
	}

	for k, v := range parser.Globals {
		cp.Globals[k] = v
	}

	cp.Targets = make([]string, len(parser.Targets))
	copy(cp.Targets, parser.Targets)

	return
}

func (parser *Arguments) DelArg(options ...string) {
	for _, option := range options {
		delete(parser.Options, option)
		delete(parser.Globals, option)
	}
}

func (parser *Arguments) NeedRoot(runtime *Runtime) bool {
	if parser.ExistsArg("h", "help") {
		return false
	}

	switch parser.Op {
	case "D", "database":
		if parser.ExistsArg("k", "check") {
			return false
		}
		return true
	case "F", "files":
		if parser.ExistsArg("y", "refresh") {
			return true
		}
		return false
	case "Q", "query":
		if parser.ExistsArg("k", "check") {
			return true
		}
		return false
	case "R", "remove":
		if parser.ExistsArg("p", "print", "print-format") {
			return false
		}
		return true
	case "S", "sync":
		if parser.ExistsArg("y", "refresh") {
			return true
		}
		if parser.ExistsArg("p", "print", "print-format") {
			return false
		}
		if parser.ExistsArg("s", "search") {
			return false
		}
		if parser.ExistsArg("l", "list") {
			return false
		}
		if parser.ExistsArg("g", "groups") {
			return false
		}
		if parser.ExistsArg("i", "info") {
			return false
		}
		if parser.ExistsArg("c", "clean") && runtime.Mode == ModeAUR {
			return false
		}
		return true
	case "U", "upgrade":
		return true
	default:
		return false
	}
}

func (parser *Arguments) addOP(op string) error {
	if parser.Op != "" {
		return errors.New(gotext.Get("only one operation may be used at a time"))
	}

	parser.Op = op
	return nil
}

func (parser *Arguments) addParam(option, arg string) error {
	if !isArg(option) {
		return errors.New(gotext.Get("invalid option '%s'", option))
	}

	if isOp(option) {
		return parser.addOP(option)
	}

	if isGlobal(option) {
		if parser.Globals[option] == nil {
			parser.Globals[option] = &Option{}
		}
		parser.Globals[option].Add(arg)
	} else {
		if parser.Options[option] == nil {
			parser.Options[option] = &Option{}
		}
		parser.Options[option].Add(arg)
	}
	return nil
}

func (parser *Arguments) AddArg(options ...string) error {
	for _, option := range options {
		err := parser.addParam(option, "")
		if err != nil {
			return err
		}
	}
	return nil
}

// Multiple args acts as an OR operator
func (parser *Arguments) ExistsArg(options ...string) bool {
	for _, option := range options {
		_, exists := parser.Options[option]
		if exists {
			return true
		}

		_, exists = parser.Globals[option]
		if exists {
			return true
		}
	}
	return false
}

func (parser *Arguments) GetArg(options ...string) (arg string, double, exists bool) {
	for _, option := range options {
		value, exists := parser.Options[option]
		if exists {
			return value.First(), len(value.Args) >= 2, len(value.Args) >= 1
		}

		value, exists = parser.Globals[option]
		if exists {
			return value.First(), len(value.Args) >= 2, len(value.Args) >= 1
		}
	}

	return arg, false, false
}

func (parser *Arguments) AddTarget(targets ...string) {
	parser.Targets = append(parser.Targets, targets...)
}

func (parser *Arguments) ClearTargets() {
	parser.Targets = make([]string, 0)
}

// Multiple args acts as an OR operator
func (parser *Arguments) ExistsDouble(options ...string) bool {
	for _, option := range options {
		value, exists := parser.Options[option]
		if exists {
			return len(value.Args) >= 2
		}

		value, exists = parser.Globals[option]
		if exists {
			return len(value.Args) >= 2
		}
	}

	return false
}

func (parser *Arguments) FormatArgs() (args []string) {
	var op string

	if parser.Op != "" {
		op = formatArg(parser.Op)
	}

	args = append(args, op)

	for option, arg := range parser.Options {
		if option == "--" {
			continue
		}

		formattedOption := formatArg(option)

		for _, value := range arg.Args {
			args = append(args, formattedOption)
			if hasParam(option) {
				args = append(args, value)
			}
		}
	}

	return
}

func (parser *Arguments) FormatGlobals() (args []string) {
	for option, arg := range parser.Globals {
		formattedOption := formatArg(option)

		for _, value := range arg.Args {
			args = append(args, formattedOption)
			if hasParam(option) {
				args = append(args, value)
			}
		}
	}

	return args
}

func formatArg(arg string) string {
	if len(arg) > 1 {
		arg = "--" + arg
	} else {
		arg = "-" + arg
	}

	return arg
}

func isArg(arg string) bool {
	switch arg {
	case "-", "--":
	case "ask":
	case "D", "database":
	case "Q", "query":
	case "R", "remove":
	case "S", "sync":
	case "T", "deptest":
	case "U", "upgrade":
	case "F", "files":
	case "V", "version":
	case "h", "help":
	case "Y", "yay":
	case "P", "show":
	case "G", "getpkgbuild":
	case "b", "dbpath":
	case "r", "root":
	case "v", "verbose":
	case "arch":
	case "cachedir":
	case "color":
	case "config":
	case "debug":
	case "gpgdir":
	case "hookdir":
	case "logfile":
	case "noconfirm":
	case "confirm":
	case "disable-download-timeout":
	case "sysroot":
	case "d", "nodeps":
	case "assume-installed":
	case "dbonly":
	case "absdir":
	case "noprogressbar":
	case "noscriptlet":
	case "p", "print":
	case "print-format":
	case "asdeps":
	case "asexplicit":
	case "ignore":
	case "ignoregroup":
	case "needed":
	case "overwrite":
	case "f", "force":
	case "c", "changelog":
	case "deps":
	case "e", "explicit":
	case "g", "groups":
	case "i", "info":
	case "k", "check":
	case "l", "list":
	case "m", "foreign":
	case "n", "native":
	case "o", "owns":
	case "file":
	case "q", "quiet":
	case "s", "search":
	case "t", "unrequired":
	case "u", "upgrades":
	case "cascade":
	case "nosave":
	case "recursive":
	case "unneeded":
	case "clean":
	case "sysupgrade":
	case "w", "downloadonly":
	case "y", "refresh":
	case "x", "regex":
	case "machinereadable":
	// yay options
	case "aururl":
	case "save":
	case "afterclean", "cleanafter":
	case "noafterclean", "nocleanafter":
	case "devel":
	case "nodevel":
	case "timeupdate":
	case "notimeupdate":
	case "topdown":
	case "bottomup":
	case "completioninterval":
	case "sortby":
	case "searchby":
	case "redownload":
	case "redownloadall":
	case "noredownload":
	case "rebuild":
	case "rebuildall":
	case "rebuildtree":
	case "norebuild":
	case "batchinstall":
	case "nobatchinstall":
	case "answerclean":
	case "noanswerclean":
	case "answerdiff":
	case "noanswerdiff":
	case "answeredit":
	case "noansweredit":
	case "answerupgrade":
	case "noanswerupgrade":
	case "gpgflags":
	case "mflags":
	case "gitflags":
	case "builddir":
	case "editor":
	case "editorflags":
	case "makepkg":
	case "makepkgconf":
	case "nomakepkgconf":
	case "pacman":
	case "git":
	case "gpg":
	case "sudo":
	case "sudoflags":
	case "requestsplitn":
	case "sudoloop":
	case "nosudoloop":
	case "provides":
	case "noprovides":
	case "pgpfetch":
	case "nopgpfetch":
	case "upgrademenu":
	case "noupgrademenu":
	case "cleanmenu":
	case "nocleanmenu":
	case "diffmenu":
	case "nodiffmenu":
	case "editmenu":
	case "noeditmenu":
	case "useask":
	case "nouseask":
	case "combinedupgrade":
	case "nocombinedupgrade":
	case "a", "aur":
	case "repo":
	case "removemake":
	case "noremovemake":
	case "askremovemake":
	case "complete":
	case "stats":
	case "news":
	case "gendb":
	case "currentconfig":
	default:
		return false
	}

	return true
}

func handleConfig(config *Configuration, option, value string) bool {
	switch option {
	case "aururl":
		config.AURURL = value
	case "save":
		config.Runtime.SaveConfig = true
	case "afterclean", "cleanafter":
		config.CleanAfter = true
	case "noafterclean", "nocleanafter":
		config.CleanAfter = false
	case "devel":
		config.Devel = true
	case "nodevel":
		config.Devel = false
	case "timeupdate":
		config.TimeUpdate = true
	case "notimeupdate":
		config.TimeUpdate = false
	case "topdown":
		config.SortMode = TopDown
	case "bottomup":
		config.SortMode = BottomUp
	case "completioninterval":
		n, err := strconv.Atoi(value)
		if err == nil {
			config.CompletionInterval = n
		}
	case "sortby":
		config.SortBy = value
	case "searchby":
		config.SearchBy = value
	case "noconfirm":
		config.NoConfirm = true
	case "config":
		config.PacmanConf = value
	case "redownload":
		config.ReDownload = "yes"
	case "redownloadall":
		config.ReDownload = "all"
	case "noredownload":
		config.ReDownload = "no"
	case "rebuild":
		config.ReBuild = "yes"
	case "rebuildall":
		config.ReBuild = "all"
	case "rebuildtree":
		config.ReBuild = "tree"
	case "norebuild":
		config.ReBuild = "no"
	case "batchinstall":
		config.BatchInstall = true
	case "nobatchinstall":
		config.BatchInstall = false
	case "answerclean":
		config.AnswerClean = value
	case "noanswerclean":
		config.AnswerClean = ""
	case "answerdiff":
		config.AnswerDiff = value
	case "noanswerdiff":
		config.AnswerDiff = ""
	case "answeredit":
		config.AnswerEdit = value
	case "noansweredit":
		config.AnswerEdit = ""
	case "answerupgrade":
		config.AnswerUpgrade = value
	case "noanswerupgrade":
		config.AnswerUpgrade = ""
	case "gpgflags":
		config.GpgFlags = value
	case "mflags":
		config.MFlags = value
	case "gitflags":
		config.GitFlags = value
	case "builddir":
		config.BuildDir = value
	case "absdir":
		config.ABSDir = value
	case "editor":
		config.Editor = value
	case "editorflags":
		config.EditorFlags = value
	case "makepkg":
		config.MakepkgBin = value
	case "makepkgconf":
		config.MakepkgConf = value
	case "nomakepkgconf":
		config.MakepkgConf = ""
	case "pacman":
		config.PacmanBin = value
	case "git":
		config.GitBin = value
	case "gpg":
		config.GpgBin = value
	case "sudo":
		config.SudoBin = value
	case "sudoflags":
		config.SudoFlags = value
	case "requestsplitn":
		n, err := strconv.Atoi(value)
		if err == nil && n > 0 {
			config.RequestSplitN = n
		}
	case "sudoloop":
		config.SudoLoop = true
	case "nosudoloop":
		config.SudoLoop = false
	case "provides":
		config.Provides = true
	case "noprovides":
		config.Provides = false
	case "pgpfetch":
		config.PGPFetch = true
	case "nopgpfetch":
		config.PGPFetch = false
	case "upgrademenu":
		config.UpgradeMenu = true
	case "noupgrademenu":
		config.UpgradeMenu = false
	case "cleanmenu":
		config.CleanMenu = true
	case "nocleanmenu":
		config.CleanMenu = false
	case "diffmenu":
		config.DiffMenu = true
	case "nodiffmenu":
		config.DiffMenu = false
	case "editmenu":
		config.EditMenu = true
	case "noeditmenu":
		config.EditMenu = false
	case "useask":
		config.UseAsk = true
	case "nouseask":
		config.UseAsk = false
	case "combinedupgrade":
		config.CombinedUpgrade = true
	case "nocombinedupgrade":
		config.CombinedUpgrade = false
	case "a", "aur":
		config.Runtime.Mode = ModeAUR
	case "repo":
		config.Runtime.Mode = ModeRepo
	case "removemake":
		config.RemoveMake = "yes"
	case "noremovemake":
		config.RemoveMake = "no"
	case "askremovemake":
		config.RemoveMake = "ask"
	default:
		return false
	}

	return true
}

func isOp(op string) bool {
	switch op {
	case "V", "version":
	case "D", "database":
	case "F", "files":
	case "Q", "query":
	case "R", "remove":
	case "S", "sync":
	case "T", "deptest":
	case "U", "upgrade":
	// yay specific
	case "Y", "yay":
	case "P", "show":
	case "G", "getpkgbuild":
	default:
		return false
	}

	return true
}

func isGlobal(op string) bool {
	switch op {
	case "b", "dbpath":
	case "r", "root":
	case "v", "verbose":
	case "arch":
	case "cachedir":
	case "color":
	case "config":
	case "debug":
	case "gpgdir":
	case "hookdir":
	case "logfile":
	case "noconfirm":
	case "confirm":
	default:
		return false
	}

	return true
}

func hasParam(arg string) bool {
	switch arg {
	case "dbpath", "b":
	case "root", "r":
	case "sysroot":
	case "config":
	case "ignore":
	case "assume-installed":
	case "overwrite":
	case "ask":
	case "cachedir":
	case "hookdir":
	case "logfile":
	case "ignoregroup":
	case "arch":
	case "print-format":
	case "gpgdir":
	case "color":
	// yay params
	case "aururl":
	case "mflags":
	case "gpgflags":
	case "gitflags":
	case "builddir":
	case "absdir":
	case "editor":
	case "editorflags":
	case "makepkg":
	case "makepkgconf":
	case "pacman":
	case "git":
	case "gpg":
	case "sudo":
	case "sudoflags":
	case "requestsplitn":
	case "answerclean":
	case "answerdiff":
	case "answeredit":
	case "answerupgrade":
	case "completioninterval":
	case "sortby":
	case "searchby":
	default:
		return false
	}

	return true
}

// Parses short hand options such as:
// -Syu -b/some/path -
func (parser *Arguments) parseShortOption(arg, param string) (usedNext bool, err error) {
	if arg == "-" {
		err = parser.AddArg("-")
		return
	}

	arg = arg[1:]

	for k, _char := range arg {
		char := string(_char)

		if hasParam(char) {
			if k < len(arg)-1 {
				err = parser.addParam(char, arg[k+1:])
			} else {
				usedNext = true
				err = parser.addParam(char, param)
			}

			break
		} else {
			err = parser.AddArg(char)

			if err != nil {
				return
			}
		}
	}

	return
}

// Parses full length options such as:
// --sync --refresh --sysupgrade --dbpath /some/path --
func (parser *Arguments) parseLongOption(arg, param string) (usedNext bool, err error) {
	if arg == "--" {
		err = parser.AddArg(arg)
		return
	}

	arg = arg[2:]

	switch split := strings.SplitN(arg, "=", 2); {
	case len(split) == 2:
		err = parser.addParam(split[0], split[1])
	case hasParam(arg):
		err = parser.addParam(arg, param)
		usedNext = true
	default:
		err = parser.AddArg(arg)
	}

	return
}

func (parser *Arguments) parseStdin() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		parser.AddTarget(scanner.Text())
	}

	return os.Stdin.Close()
}

func (parser *Arguments) ParseCommandLine(config *Configuration) error {
	args := os.Args[1:]
	usedNext := false

	if len(args) < 1 {
		if _, err := parser.parseShortOption("-Syu", ""); err != nil {
			return err
		}
	} else {
		for k, arg := range args {
			var nextArg string

			if usedNext {
				usedNext = false
				continue
			}

			if k+1 < len(args) {
				nextArg = args[k+1]
			}

			var err error
			switch {
			case parser.ExistsArg("--"):
				parser.AddTarget(arg)
			case strings.HasPrefix(arg, "--"):
				usedNext, err = parser.parseLongOption(arg, nextArg)
			case strings.HasPrefix(arg, "-"):
				usedNext, err = parser.parseShortOption(arg, nextArg)
			default:
				parser.AddTarget(arg)
			}

			if err != nil {
				return err
			}
		}
	}

	if parser.Op == "" {
		parser.Op = "Y"
	}

	if parser.ExistsArg("-") {
		if err := parser.parseStdin(); err != nil {
			return err
		}
		parser.DelArg("-")

		file, err := os.Open("/dev/tty")
		if err != nil {
			return err
		}

		os.Stdin = file
	}

	parser.extractYayOptions(config)
	return nil
}

func (parser *Arguments) extractYayOptions(config *Configuration) {
	for option, value := range parser.Options {
		if handleConfig(config, option, value.First()) {
			parser.DelArg(option)
		}
	}

	for option, value := range parser.Globals {
		if handleConfig(config, option, value.First()) {
			parser.DelArg(option)
		}
	}

	rpc.AURURL = strings.TrimRight(config.AURURL, "/") + "/rpc.php?"
	config.AURURL = strings.TrimRight(config.AURURL, "/")
}