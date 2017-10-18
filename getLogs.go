// vpavesi 2017 oct 18.
// get logs by ssh/ftp.
package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

const applVersion = "version:0.0.8"

type yamlinstanceConfig struct {
	SSHHostname  string `yaml:"remote_ipaddr"`
	SSHUsername  string `yaml:"username"`
	SSHPassword  string `yaml:"password"`
	SSHKey       string `yaml:"ssh_key"`
	SSHPort      string `yaml:"ssh_port"`
	Getfiles     map[string][]string
	Exceptfiles  map[string][]string
	ExecuteTasks map[string]map[string]string
}

func (c *yamlinstanceConfig) Parse(data []byte) error {
	return yaml.Unmarshal(data, c)
}

func getKeyFile(yamlconfig yamlinstanceConfig) (key ssh.Signer, err error) {
	fileprivatekey := yamlconfig.SSHKey
	buf, err := ioutil.ReadFile(fileprivatekey)
	if err != nil {
		return
	}
	key, err = ssh.ParsePrivateKey(buf)

	if err != nil {
		return
	}
	return
}

func RealAllDirFile(sftp *sftp.Client, itemdstDirName string) (Listfilenamepathsrc []string) {
	/* get all dir and files
	 */
	var Listfilenamepath []string

	// match *.*
	if strings.Contains(itemdstDirName, "*.") {
		matches, err := sftp.Glob(itemdstDirName)
		if err != nil {
			log.Println("Glob error for %s : %s", itemdstDirName, err)
		}

		for _, matchesFileItem := range matches {
			Listfilenamepath = append(Listfilenamepath, matchesFileItem)
		}
	} else {

		listalldir, err := sftp.ReadDir(itemdstDirName)
		if err != nil {
			log.Println(err)
		}

		for _, filenamepathsrcItem := range listalldir {
			filenamepath := filepath.Join(itemdstDirName, filenamepathsrcItem.Name())
			filenamepath = filepath.ToSlash(filenamepath)

			if filenamepathsrcItem.IsDir() {
				//if dir, read again
				log.Println("dir:", filenamepathsrcItem.Name())
				ListfilenamepathsubDir := RealAllDirFile(sftp, filenamepath)
				for _, filenamepath := range ListfilenamepathsubDir {
					Listfilenamepath = append(Listfilenamepath, filenamepath)
				}
			} else {
				Listfilenamepath = append(Listfilenamepath, filenamepath)
			}

		}

	}
	return Listfilenamepath
}

func getIndividualFile(sftp *sftp.Client, itemdstDirName string, dstPath string) (err error) {
	/* func get individual file

	 */

	// Open the source file
	srcFile, err := sftp.OpenFile(itemdstDirName, os.O_RDONLY)
	if err != nil {
		log.Println(err)
		return nil
	}

	// create dir destination
	filenamepathdstFile := filepath.Base(itemdstDirName)
	filenamepathdstDir := filepath.Dir(itemdstDirName)
	newdstPath := filepath.Join(dstPath, filenamepathdstDir)

	if os.MkdirAll(newdstPath, os.ModePerm) != nil {
		panic("Unable to create directory!")
	}

	re := regexp.MustCompile(":")
	filenamepathdstFile = re.ReplaceAllLiteralString(filenamepathdstFile, "_")

	// create file destination
	dstFile, err := os.Create(filepath.Join(newdstPath, filenamepathdstFile))
	if err != nil {
		log.Fatal(err)

	}

	// Copy the file
	srcFile.WriteTo(dstFile)
	log.Println("file transferred:", filenamepathdstFile)

	// clean-up
	dstFile.Close()
	srcFile.Close()

	return err
}

func copyReturnFile(sftp *sftp.Client, itemdstDirName string, dstPath string) (err error) {
	/* func return file

	 */

	// Open the source file
	srcFile, err := sftp.OpenFile(itemdstDirName, os.O_RDONLY)
	if err != nil {
		log.Println(err)
		return nil
	}

	// create dir destination
	filenamepathdstFile := filepath.Base(itemdstDirName)

	// create file destination
	dstFile, err := os.Create(filepath.Join(dstPath, filenamepathdstFile))
	if err != nil {
		log.Fatal(err)

	}

	// Copy the file
	srcFile.WriteTo(dstFile)
	log.Println("file transferred:", filenamepathdstFile)

	// clean-up
	dstFile.Close()
	srcFile.Close()

	return err
}

func sortMapofMapsbykeys(mapin map[string]map[string]string) []string {
	/*
		func sort maps
	*/
	keys := make([]string, 0, len(mapin))
	for key := range mapin {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortMapbykeys(mapin map[string][]string) []string {
	/*
		func sort maps
	*/
	keys := make([]string, 0, len(mapin))
	for key := range mapin {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortMapStrbykeys(mapin map[string]string) []string {
	/*
		func sort maps
	*/
	keys := make([]string, 0, len(mapin))
	for key := range mapin {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

////////////////////////////
// main
func main() {
	log.Printf(applVersion)
	log.Printf("start...")

	// log file
	flogFile, err := os.OpenFile("getLogs.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}

	mw := io.MultiWriter(os.Stdout, flogFile)
	log.SetOutput(mw)

	// start time
	t := time.Now()
	dirtimename := (t.Format("20060102_150405"))
	dstPath := ("getLogs_" + dirtimename)

	startTime := time.Now()

	// read yaml config
	data, err := ioutil.ReadFile("getLogs.yml")
	if err != nil {
		log.Fatal(err)
	}
	var yamlconfig yamlinstanceConfig
	if err := yamlconfig.Parse(data); err != nil {
		log.Fatal(err)
	}
	//log.Println(yamlconfig)

	var pSSHHostname = net.ParseIP(yamlconfig.SSHHostname)
	if pSSHHostname == nil {
		log.Fatal(1)
	}

	//gey ssh keys
	key, err := getKeyFile(yamlconfig)
	if err != nil {
		log.Fatal(err)

	}

	// ssh prepare connection
	sshConfig := &ssh.ClientConfig{
		User:            yamlconfig.SSHUsername,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.Password(yamlconfig.SSHPassword), ssh.PublicKeys(key),
		},
	}

	sshConfig.SetDefaults()

	sshclient, err := ssh.Dial("tcp", yamlconfig.SSHHostname+":"+yamlconfig.SSHPort, sshConfig)
	if err != nil {
		panic("Failed to dial: " + err.Error())
	}

	log.Println("Successfully connected to ssh:", pSSHHostname)

	// open an SFTP session over an existing ssh connection.
	sftp, err := sftp.NewClient(sshclient)
	if err != nil {
		log.Fatal(err)
	}

	// count number of files  and tasks
	var icountFiles int64
	var icounttasks int64

	// var payload
	var sshpayload bytes.Buffer

	// main loop to get all files
	mapofgetfilesSort := sortMapbykeys(yamlconfig.Getfiles)
	for _, keysorted := range mapofgetfilesSort {
		ldstDirName := (yamlconfig.Getfiles[keysorted])
		log.Println("running get files:", keysorted)

		//process dir ,first
		if keysorted != "filesindividual" {

			for _, itemdstDirName := range ldstDirName {

				// get all dir/subdir/files
				Listfilenamepathsrc := RealAllDirFile(sftp, itemdstDirName)

			GOTOTOP:
				for _, filenamepathsrc := range Listfilenamepathsrc {

					// check if file exist on list of exceptions
					if yamlconfig.Exceptfiles[filenamepathsrc] != nil {
						continue GOTOTOP
					}

					// get  file name
					_, filenamepathdstFile := filepath.Split(filenamepathsrc)

					filenamepathdstFile = filepath.Base(filenamepathdstFile)

					//skip files "lost+found
					if filenamepathdstFile == "lost+found" {
						continue GOTOTOP
					}

					//  copy file from remote machine
					getIndividualFile(sftp, filenamepathsrc, dstPath)
					if err != nil {
						log.Fatal(err)
					}
					icountFiles = icountFiles + 1
				}

			}

		} else {

			// process individual files
			for _, itemdstDirName := range ldstDirName {

				//  copy file from remote machine
				getIndividualFile(sftp, itemdstDirName, dstPath)
				if err != nil {
					log.Fatal(err)
				}
				icountFiles = icountFiles + 1
			}
		}

	}

	// execute task
	mapoftasksSort := sortMapofMapsbykeys(yamlconfig.ExecuteTasks)

	// integer for iterate task
	var i64iterate int64 = -1
	var getsshlistvms []string

	for _, keysorted := range mapoftasksSort {
		mapoftasks := (yamlconfig.ExecuteTasks[keysorted])
		log.Println("running task:", keysorted)

		//taskexecute
		getlistoftasks := mapoftasks["taskexecute"]

		if getlistoftasks != "" {
			log.Println("taskexecute:", getlistoftasks)

			//non iterate
			if i64iterate == -1 {
				//sshlistvms:
				if strings.Contains(getlistoftasks, "sshlistvms:") {

					getlistoftasks := strings.Replace(getlistoftasks, "sshlistvms:", "", -1)
					log.Println("sshlistvms:", getsshlistvms)
					getlistoftasksM := strings.Split(getlistoftasks, ";")

					for _, getonevm := range getsshlistvms {

						if getonevm != "" {
							log.Println("vm:", getonevm)
							for _, getlistoftasksOne := range getlistoftasksM {

								log.Println("cmd:", getlistoftasksOne)

								// new ssh connection/execute
								sshsession, err := sshclient.NewSession()
								if err != nil {
									log.Fatal("Failed to create session: ", err)
								}
								defer sshsession.Close()

								icounttasks = icounttasks + 1
								sshsession.Stdout = &sshpayload

								// run to each vm
								getlistoftasks = "ssh " + getonevm + " " + getlistoftasksOne
								log.Println(getlistoftasks)

								if err := sshsession.Run(getlistoftasks); err != nil {
									log.Fatal("Failed to run: " + err.Error())
									log.Println(sshpayload.String())
								}
								log.Println("\n", sshpayload.String())
								sshpayload.Reset()
							}
						}
					}

					// non sshlistvms:
				} else {

					// new ssh connection/execute
					sshsession, err := sshclient.NewSession()
					if err != nil {
						log.Fatal("Failed to create session: ", err)
					}
					defer sshsession.Close()

					icounttasks = icounttasks + 1
					sshsession.Stdout = &sshpayload
					if err := sshsession.Run(getlistoftasks); err != nil {
						log.Fatal("Failed to run: " + err.Error())
						log.Println(sshpayload.String())
					}
				}
			} else {
				//iterate
				if strings.Contains(getlistoftasks, "iterate") {
					var i64 int64
					for i64 = 1; i64 <= i64iterate; i64++ {

						re := regexp.MustCompile("iterate")
						i64str := strconv.FormatInt(i64, 10)
						getlistoftasksiter := re.ReplaceAllLiteralString(getlistoftasks, i64str)
						log.Println("taskexecute:", getlistoftasksiter)

						//new ssh session/execute
						sshsession, err := sshclient.NewSession()
						if err != nil {
							log.Fatal("Failed to create session: ", err)
						}
						defer sshsession.Close()

						icounttasks = icounttasks + 1
						sshsession.Stdout = &sshpayload
						if err := sshsession.Run(getlistoftasksiter); err != nil {
							log.Fatal("Failed to run: " + err.Error())
							log.Println(sshpayload.String())
						}
						log.Println("\n", sshpayload.String())
						sshpayload.Reset()
					}
					// no more iterate
					i64iterate = -1
				}

			}

			//taskget
			getlistoftasks = mapoftasks["taskget"]
			log.Println("taskget:", getlistoftasks)

			switch getlistoftasks {

			case "console":
				log.Println("\n", sshpayload.String())
				sshpayload.Reset()

			case "none":
				sshpayload.Reset()

			case "iterate":
				taskgetiterate := sshpayload.String()
				re := regexp.MustCompile("[0-9]+")
				ntaskgetiterate := (re.FindAllString(taskgetiterate, -1))
				i64iterate, _ = strconv.ParseInt(ntaskgetiterate[len(ntaskgetiterate)-1], 10, 32)
				log.Println("iterate times:", i64iterate)

			case "return":
				// get file name returned by taskexecute
				filenameFreturn := strings.Replace(sshpayload.String(), " ", "", -1)
				filenameFreturn = strings.TrimSuffix(filenameFreturn, "\n")

				sshpayload.Reset()
				log.Println("return filename:", filenameFreturn)

				copyReturnFile(sftp, filenameFreturn, dstPath)
				if err != nil {
					log.Fatal(err)
				}
				icountFiles = icountFiles + 1

			case "sshlistvms":
				getsshlistvmsstr := strings.Replace(sshpayload.String(), " ", "", -1)
				getsshlistvms = strings.Split(getsshlistvmsstr, "\n")

			default:
				getIndividualFile(sftp, getlistoftasks, dstPath)
				if err != nil {
					log.Fatal(err)
				}
				icountFiles = icountFiles + 1
			}
		}
	}

	// close sftp
	sftp.Close()

	// how long it was running
	log.Printf("............................")
	duration := time.Since(startTime)
	log.Printf("Execution took %s", duration)
	log.Printf("total files: %v", icountFiles)
	log.Printf("total tasks: %v", icounttasks)

	log.Printf("...end")
	os.Exit(3)
}
