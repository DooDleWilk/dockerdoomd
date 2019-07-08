package main

//TODO: Make your container die if you die

import (
        "flag"
        "fmt"
        "log"
        "net"
        "os"
        "os/exec"
        "strings"
        "time"
)

func runCmd(cmdstring string) {
        parts := strings.Split(cmdstring, " ")
        cmd := exec.Command(parts[0], parts[1:len(parts)]...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        err := cmd.Run()
        if err != nil {
                log.Fatalf("The following command failed: \"%v\"\n", cmdstring)
        }
}

func outputCmd(cmdstring string) string {
        parts := strings.Split(cmdstring, " ")
        cmd := exec.Command(parts[0], parts[1:len(parts)]...)
        cmd.Stderr = os.Stderr
        output, err := cmd.Output()
        if err != nil {
                log.Fatalf("The following command failed: \"%v\"\n", cmdstring)
        }
        return string(output)
}

func startCmd(cmdstring string) {
        parts := strings.Split(cmdstring, " ")
        cmd := exec.Command(parts[0], parts[1:len(parts)]...)
        cmd.Stdout = os.Stdout
        cmd.Stdin = os.Stdin
        err := cmd.Start()
        if err != nil {
                log.Fatalf("The following command failed: \"%v\"\n", cmdstring)
        }
}

func checkDockerImages(imageName, dockerBinary, dockerHostString string) bool {
        output := outputCmd(fmt.Sprintf("%v %v images -q %v", dockerBinary, dockerHostString, imageName))
        return len(output) > 0
}

func checkActiveDocker(dockerName, dockerBinary, dockerHostString string) bool {
        return checkDocker(dockerName, dockerHostString, dockerBinary, "-q")
}

func checkAllDocker(dockerName, dockerBinary, dockerHostString string) bool {
        return checkDocker(dockerName, dockerHostString, dockerBinary, "-aq")
}

func checkDocker(dockerName, dockerHostString, dockerBinary, arg string) bool {
        output := outputCmd(fmt.Sprintf("%v %v ps %v", dockerBinary, dockerHostString, arg))
        docker_ids := strings.Split(string(output), "\n")
        for _, docker_id := range docker_ids {
                if len(docker_id) == 0 {
                        continue
                }
                output := outputCmd(fmt.Sprintf("%v %v inspect -f {{.Name}} %v", dockerBinary, dockerHostString, docker_id))
                name := strings.TrimSpace(string(output))
                name = name[1:len(name)]
                if name == dockerName {
                        return true
                }
        }
        return false
}

func socketLoop(listener net.Listener, dockerBinary, dockerHostString, containerName string) {
        for true {
                conn, err := listener.Accept()
                if err != nil {
                        panic(err)
                }
                stop := false
                for !stop {
                        bytes := make([]byte, 40960)
                        n, err := conn.Read(bytes)
                        if err != nil {
                                stop = true
                        }
                        bytes = bytes[0:n]
                        strbytes := strings.TrimSpace(string(bytes))
                        if strbytes == "list" {
                                output := outputCmd(fmt.Sprintf("%v %v ps -q", dockerBinary, dockerHostString))
                                //cmd := exec.Command("/usr/bin/docker", "inspect", "-f", "{{.Name}}", "`docker", "ps", "-q`")
                                outputstr := strings.TrimSpace(output)
                                outputparts := strings.Split(outputstr, "\n")
                                for _, part := range outputparts {
                                        output := outputCmd(fmt.Sprintf("%v %v inspect -f {{.Name}} %v", dockerBinary, dockerHostString, part))
                                        name := strings.TrimSpace(output)
                                        name = name[1:len(name)]
                                        if name != containerName {
                                                _, err = conn.Write([]byte(name + "\n"))
                                                if err != nil {
                                                        log.Fatal("Could not write to socket file")
                                                }
                                        }
                                }
                                conn.Close()
                                stop = true
                        } else if strings.HasPrefix(strbytes, "kill ") {
                                parts := strings.Split(strbytes, " ")
                                docker_id := strings.TrimSpace(parts[1])
                                startCmd(fmt.Sprintf("%v %v rm -f %v", dockerBinary, dockerHostString, docker_id))
                                //cmd := exec.Command(dockerBinary, "rm", "-f", docker_id)
                                //go cmd.Run()
                                conn.Close()
                                stop = true
                        }
                }
        }
}

func main() {
        var socketFileFormat, socketFileType, containerName, vncPort, dockerBinary, imageName, dockerfile, dockerHost, dockerHostString, sshPortString string
        var dockerWait int
        var buildImage, asciiDisplay bool
        flag.StringVar(&socketFileType, "socketFileType", "file", "Socket file type : FILE or SSH")
        flag.StringVar(&socketFileFormat, "socketFileFormat", "/tmp/dockerdoom%v.socket", "Location and format of the socket file")
        flag.StringVar(&containerName, "containerName", "dockerdoom", "Name of the docker container running DOOM")
        flag.IntVar(&dockerWait, "dockerWait", 5, "Time to wait before checking if the container came up")
        flag.StringVar(&vncPort, "vncPort", "5900", "Port to open for VNC Viewer")
        flag.StringVar(&dockerBinary, "dockerBinary", "/usr/bin/docker", "docker binary")
        flag.BoolVar(&buildImage, "buildImage", false, "Build docker image instead of pulling it from docker image repo")
        flag.StringVar(&imageName, "imageName", "gideonred/dockerdoom", "Name of docker image to use")
        flag.StringVar(&dockerfile, "dockerfile", ".", "Path to dockerdoom's Dockerfile")
        flag.BoolVar(&asciiDisplay, "asciiDisplay", false, "Don't use fancy vnc, throw DOOM straightup on my terminal screen")
        flag.StringVar(&dockerHost, "dockerHost", "", "docker host")
        flag.Parse()

        dockerHostString = ""
        if len(dockerHost) != 0 {
                dockerHostString = "-H " + dockerHost
        }

        present := checkDockerImages(imageName, dockerBinary, dockerHostString)
        if !present {
                log.Print("Pulling image : %v", imageName)
                runCmd(fmt.Sprintf("%v %v pull %v", dockerBinary, dockerHostString, imageName))
                log.Print("Image downloaded")
        }

        present = checkAllDocker(containerName, dockerBinary, dockerHostString)
        if present {
                log.Fatalf("\"%v\" was present in the output of \"docker ps -a\",\nplease remove before trying again. You could use \"docker %v rm -f %v\"\n", containerName, dockerHostString, containerName)
        }

        socketFile := fmt.Sprintf(socketFileFormat, time.Now().Unix())
        listener, err := net.Listen("unix", socketFile)
        if err != nil {
                log.Fatalf("Could not create socket file %v.\nYou could use \"rm -f %v\"", socketFile, socketFile)
        }


        sshPortString = ""
        if socketFileType == "ssh" {
                log.Print("Setting up forward Unix domain socket over SSH")
                sshSocketRun := fmt.Sprintf("ssh -nNT -L %v:/dockerdoom.socket root@%v -p 8022", socketFile, dockerHost)
                startCmd(sshSocketRun)
                sshPortString = "-p 8022:22"
        }

        log.Print("Trying to start docker container ...")
        if !asciiDisplay {
                dockerRun := fmt.Sprintf("%v %v run --rm=true %v -p %v:%v -v %v:/dockerdoom.socket --name=%v %v x11vnc -geometry 640x480 -forever -usepw -create", dockerBinary, dockerHostString, sshPortString, vncPort, vncPort, socketFile, containerName, imageName)
                startCmd(dockerRun)
                log.Printf("Waiting %v seconds for \"%v\" to show in \"docker %v ps\". You can change this wait with -dockerWait.", dockerWait, containerName, dockerHostString)
                time.Sleep(time.Duration(dockerWait) * time.Second)
                present = checkActiveDocker(containerName, dockerBinary, dockerHostString)
                if !present {
                        log.Fatalf("\"%v\" did not lead to the container appearing in \"docker %v ps\". Please try and start it manually and check \"docker %v ps\"\n", dockerRun, dockerHostString, dockerHostString)
                }
                log.Print("Docker container started, you can now connect to it with a VNC viewer at port 5900")
        } else {
                dockerRun := fmt.Sprintf("%v %v run -t -i --rm=true -p %v:%v -v %v:/dockerdoom.socket --name=%v %v /bin/bash", dockerBinary, dockerHostString, vncPort, vncPort, socketFile, containerName, imageName)
                startCmd(dockerRun)
        }

        socketLoop(listener, dockerBinary, dockerHostString, containerName)
}
