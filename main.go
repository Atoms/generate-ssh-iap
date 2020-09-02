package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"text/template"

	homedir "github.com/mitchellh/go-homedir"
	flag "github.com/spf13/pflag"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

// SSHInventory (all needed fields)
type SSHInventory struct {
	VM                 string
	Project            string
	Zone               string
	ComputeID          uint64
	SSHKeyFile         string
	SSHKKnownHostsFile string
	GcloudPy           string
	SSHUsername        string
}

//Usage prints help message
var Usage = func() {
	fmt.Fprintf(os.Stderr, "\nUsage of %s:\n", os.Args[0])
	fmt.Printf("\n")
	flag.PrintDefaults()
	fmt.Printf("\n")
}

var sshuser = ""

func main() {

	projectID := flag.StringP("project", "p", "", "Select project where instance lives")
	zone := flag.StringP("zone", "z", "", "Select zone where instance is located")

	vmname := flag.StringP("vmname", "v", "", "Select VM for which to generate ssh config")
	sshusername := flag.StringP("user", "u", "", "Use different username")

	flag.CommandLine.SortFlags = false
	flag.Parse()

	if (*projectID == "") || (*zone == "") || (*vmname == "") {
		Usage()
		os.Exit(0)
	}

	tmpldef := `

----
Copy below host definition to .ssh/config file
----

Host {{ .VM }}
    HostName compute.{{ .ComputeID }}
    IdentityFile {{ .SSHKeyFile }}
    CheckHostIP no
    HostKeyAlias compute.{{ .ComputeID }}
    IdentitiesOnly yes
    StrictHostKeyChecking yes
    UserKnownHostsFile {{ .SSHKKnownHostsFile }}
    ProxyCommand python -S {{ .GcloudPy }} compute start-iap-tunnel {{ .VM }} %p --listen-on-stdin --project={{ .Project }} --zone={{ .Zone }} --verbosity=warning
    ProxyUseFdpass no
    ForwardAgent yes
    User {{ .SSHUsername }}
    ControlMaster auto
    ControlPersist 30
    PreferredAuthentications publickey
    KbdInteractiveAuthentication no
    PasswordAuthentication no
    ConnectTimeout 20
    ControlPath /tmp/ssh-{{ .VM }}-iap

`

	GoogleCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	if len(GoogleCreds) == 0 {
		fmt.Println("ERR: add to environment GOOGLE_APPLICATION_CREDENTIALS variable with your service account credentials")
		os.Exit(2)
	}

	ctx := context.Background()

	c, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}
	computeService, err := compute.New(c)
	if err != nil {
		log.Fatal(err)
	}
	if *sshusername == "" {
		user, err := user.Current()
		if err != nil {
			log.Fatalf(err.Error())
		}
		sshuser = user.Username
	} else {
		sshuser = *sshusername
	}
	filter := fmt.Sprintf("name=%s", *vmname)
	gcloudpylocation := getgcloudpath()

	req := computeService.Instances.List(*projectID, *zone).Filter(filter)
	if err := req.Pages(ctx, func(page *compute.InstanceList) error {
		for _, instance := range page.Items {
			var sshdata SSHInventory
			sshdata.ComputeID = uint64(instance.Id)
			sshdata.VM = *vmname
			sshdata.SSHKeyFile = getSSHKeyFile()
			sshdata.SSHKKnownHostsFile = getSSHKKnownHostsFile()
			sshdata.Project = *projectID
			sshdata.Zone = *zone
			sshdata.GcloudPy = gcloudpylocation
			sshdata.SSHUsername = sshuser
			tmpl, err := template.New("ssh_config").Parse(tmpldef)
			if err != nil {
				panic(err)
			}
			err = tmpl.Execute(os.Stdout, sshdata)
			if err != nil {
				panic(err)
			}
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}

func getgcloudpath() string {
	path, err := exec.LookPath("gcloud")
	if err != nil {
		log.Fatal("installing gcloud is in your future")
	}
	return filepath.Dir(filepath.Dir(path)) + "/lib/gcloud.py"
}

func getSSHKeyFile() string {
	sshkeyfile := "~/.ssh/id_rsa"

	var SSHKeyFilePath = ""
	SSHKeyFilePath, err := homedir.Expand(sshkeyfile)
	if err != nil {
		log.Fatal(err)
	}
	return SSHKeyFilePath
}
func getSSHKKnownHostsFile() string {
	sshkknownhostsfile := "~/.ssh/google_compute_known_hosts"

	var SSHKKnownHostsFile = ""
	SSHKKnownHostsFile, err := homedir.Expand(sshkknownhostsfile)
	if err != nil {
		log.Fatal(err)
	}
	return SSHKKnownHostsFile
}
