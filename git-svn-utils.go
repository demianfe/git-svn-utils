package main

/*
  Compilar con:
    GOOS=darwin GOARCH=amd64 go build hello.go
*/
import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"encoding/json"
)

const IGNORE_FILE = ".gitignore"
var conf Configuration = readConfiguration()
//singleRepo = conf.SingleRepo

type Configuration struct {
	SvnUrl string
	SvnDir string
	GitDir string
	SingleRepo bool
}

func readConfiguration() (Configuration){
	file, _ := os.Open("conf.json")
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("Error al abrir el archivo.")
		log.Fatal(err)
	}
	return configuration
}

/*
TODO: manejar difrencias upstream y locales.
pasos a seguir para comitear cambios
  - git commit localmente 
  - git svn dcommit 
Si hay conflicto
  - git svn rebase
  - git rebase --continue
  - git svn dcommit
*/
func addTrailingSlash(path string) string {
	if strings.Index(path, "/") != len(path) {
		path = path + "/"
	}
	return path
}

/*
Recibe una lista de comandos a ser ejecutados
y retorna una lista de lineas de output
*/
func executeCommand(args ...string) []string {
	head := args[0]
	parts := args[1:len(args)]
	cmd := exec.Command(head, parts...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Println("Error: ", err)
	}
	outLines := strings.Split(out.String(), "\n")
	return outLines
}

func executeCommandOnDir(dir string, args ...string) []string{
	error := os.Chdir(dir)
	if error != nil {
		log.Fatal(error)
	}
	outLines := executeCommand(args...)
	return outLines
}

/*
recorre los sub directorios git y ejecuta el comando
pasado como parametro
*/
func walkAndExecuteCommand(gitDir string, args ...string) map[string]([]string) {
	files, _ := ioutil.ReadDir(gitDir)
	results := make(map[string]([]string))

	for _, f := range files {
		if f.IsDir() {
			error := os.Chdir(gitDir + "/" + f.Name())
			if error != nil {
				log.Fatal(error)
			}
			outLines := executeCommand(args...)
			results[f.Name()] = outLines
		}
	}
	return results
}

/*
clona los repositorios de subversion
y luego copia los archivos del zip
*/
func gitSVNClone(svnUrl string, localRepo string, gitRepo string) {
	//iterar en la copia local: NO TIENE QUE HABER NINGUN CAMBIO SIN PUBLICARSE
	//clonar cada item de la iteracion con git-svn
	//TODO ignorar ciertos directorios .svn 
	files, _ := ioutil.ReadDir(localRepo)
	for _, f := range files {
		if f.IsDir() {
			fmt.Println("Ejecutando el comando: ")	
			fmt.Println("git svn clone " + 
				svnUrl + f.Name(),
				gitRepo+f.Name())
			outLines := executeCommand("git",
				"svn",
				"clone",
				svnUrl + f.Name(),
				gitRepo + f.Name())
			for _, line := range outLines {
				fmt.Println(line)
			}
		}
	}
}

func overrideClonedRepository(svnDir string, gitDir string){
	fmt.Println("Copiando y sobreescribiendo con los datos locales.")
	svnDir = addTrailingSlash(svnDir)
	gitDir = addTrailingSlash(gitDir)	
	//iterar por los directorios y copiarlos de a uno
	files, _ := ioutil.ReadDir(gitDir)
	for _, f := range files {
		fmt.Println("cp", "-rf ", svnDir + f.Name(), gitDir)
		cpCmd := exec.Command("cp", "-rf", svnDir + f.Name(), gitDir)
		cpErr := cpCmd.Run()
		if cpErr != nil {
			log.Println("Error: ", cpErr)
		}
	}
}

/* sobreescribte todos los archivos del repositorio
con la copia actual proveniente del zip*/
func removeSVNData(svnDir string, gitDir string) {
	fmt.Println("eliminando svn metadata")
	files, _ := ioutil.ReadDir(gitDir)
	for _, f := range files {
		if f.IsDir() {
			error := os.Chdir(gitDir + "/" + f.Name())
			if error != nil {
				log.Fatal(error)
			}
			currentDir, error := os.Getwd()
			gitCmdStr := fmt.Sprintf("rm -rf $(find %s/ -type d -name .svn", currentDir)
			rmCmd := exec.Command(gitCmdStr)
			rmErr := rmCmd.Run()
			if rmErr != nil {
				log.Println("Error: ", rmErr)
			}
		}
	}
}

func listGitChanges(gitDir string) {
	//recorre los subdirectorios e imprime los cambios en cada subproyecto git
	if(conf.SingleRepo){
		results := executeCommandOnDir(gitDir, "git", "status", "-s")
		for _, output := range results {
			fmt.Println("-> ", output)  
		}
	}else{
		results := walkAndExecuteCommand(gitDir, "git", "status", "-s")
		for gitRepo, outLines := range results{
			//ignoramos este repo si no hay cambios
			if len(outLines) > 2 && outLines[1] != "nothing to commit, working directory clean" {
				fmt.Println("Cambios en: " + gitRepo)
				fmt.Println("---------------------------------")
				for index, output := range outLines{
					if index == 0 {
						fmt.Println("-> ", output)  
					}else{
						fmt.Println(output)					
					}
				}
				fmt.Println("")
			}
		}
	}
}

// codigo nim
// proc revertAssumeUnchangedFiles() =
//   #chequear el archivo con las lista de archivos marcados con assume unchanged
//   #iterar sobre la lista y ejecutar --no-assume-unchanged por cada item
//   for dirItem in walkDir GIT_DIR:
//     if existsDir dirItem.path:
//       setCurrentDir dirItem.path
//       echo "reading $1"  %  [dirItem.path]
//       var outp = execProcess "git ls-files -v | grep '^[[:lower:]]'"
//       for line in outp.split "\n":
//         echo line
//         # var outp = execProcess "git update-index --no-assume-unchanged $1" % [fileName]
// # echo outp

func revertAssumeUnchangedFiles() {
//TODO:
}

func manageFilesToIgnore(dir string) {
	error := os.Chdir(dir)
	if error != nil {
		log.Fatal(error)
	}
	var results []string
	outLines := executeCommand("git", "status", "-s")
	for _, outline := range outLines {
		line := strings.TrimSpace(outline)
		if strings.Index(line, "D") == 0 {
			//D en la primera posicion
			fmt.Println("deleted file -->", line)
		} else if strings.Index(line, "D") == 0 {
			fmt.Println("git",
				"update-index",
				"--assume-unchanged",
				line[2: len(line)])
			results = executeCommand("git",
				"update-index",
				"--assume-unchanged",
				line[2: len(line)])
		}else if strings.Index(line, "M") == 0 {
			fmt.Println("git",
				"update-index",
				"--assume-unchanged",
				line[2: len(line)])
			results = executeCommand("git",
				"update-index",
				"--assume-unchanged",
				line[2: len(line)])
			//fmt.Println("Changed file -->", line)
		} else if strings.Index(line, "??") == 0 {
			//?? para archivos nuevos
			//escribimos a .gitignore
			fmt.Println("TODO")
		}
		for _, result:= range results{
			fmt.Println(result)
		}
	}
}

func tuneGITRepo(gitDir string) {
	if(conf.SingleRepo) {
		manageFilesToIgnore(gitDir)
	}else {
		files, _ := ioutil.ReadDir(gitDir)
		for _, f := range files {
			if f.IsDir() {
				manageFilesToIgnore(gitDir + "/" + f.Name())
			}
		}
	}
}

func setupLocalRepo(svnUrl string, localRepo string, gitRepo string) {
	localRepo = addTrailingSlash(localRepo)
	gitRepo = addTrailingSlash(gitRepo)
	gitSVNClone(svnUrl, localRepo, gitRepo)
	//overrideClonedRepository(localRepo, gitRepo)
}

func updateLocalRepo(gitDir string){
	//ejecuta git svn rebase en cada repo
	if(conf.SingleRepo){
		results := executeCommandOnDir(gitDir, "git", "svn", "rebase")
		for _, output := range results {
			fmt.Println("-> ", output)
		}
	}else{
		results := walkAndExecuteCommand(gitDir, "git", "svn", "rebase")
		fmt.Println(results)
	}
}

func printHelp() {
	fmt.Println("Uso")
	fmt.Println("--help\t\t\t\timprime este mensaje")
	fmt.Println("--crear-repo\t\t\tclona con git-svn y luego copia los archivos del zip")
	fmt.Println("--ignorar-cambios\t\ttrata los diferentes casos de cambios locales que no deben subir upstream.")
	fmt.Println("--listar-ignorados\t\tlista los archivos cambiados que se estan ignorando.")
	fmt.Println("--agregar-archivo\t\tagrega un archivo previamente ignorado al versionado.")
	fmt.Println("--listar-cambios\t\tlista todos los cambios en los diferentes subrepositiorios.")
}

func main() {
	var mainCmd string

	if len(os.Args) <= 1 {
		mainCmd = "--help"
	} else {
		mainCmd = os.Args[1]
	}
	
	switch mainCmd {
	case "--help":
		printHelp()
	case "--crear-repo":
		setupLocalRepo(conf.SvnUrl, conf.SvnDir, conf.GitDir)
	case "--ignorar-cambios":
		//fmt.Println("TODO: unimplemented command.")
		tuneGITRepo(conf.GitDir)
	case "--listar-ignorados":
		fmt.Println("TODO: unimplemented command.")
	case "--agregar-archivo":
		fmt.Println("TODO: unimplemented command.")
	case "--listar-cambios":
		listGitChanges(conf.GitDir)
	case "--sobreescribir-repo-git":
		overrideClonedRepository(conf.SvnDir, conf.GitDir)
		//removeSVNData(conf.SvnDir, conf.GitDir)
	case "--actualizar":
		updateLocalRepo(conf.GitDir)
	default:
		fmt.Println("Error: argumentos equivocados.")
		fmt.Println("")
		printHelp()
	}
}
