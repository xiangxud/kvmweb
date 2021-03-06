package kvm

import (
	"fmt"
	"io"
	"time"

	kvmconf "github.com/pion/rtsp-bench/server/config"
	"github.com/pion/webrtc/v3"
	"golang.org/x/crypto/ssh"
)

func initSSH(sshUser, sshPassword, sshAddr string, dc *webrtc.DataChannel, rtcin chan string) (*ssh.Session, error) {
	//创建sshp登陆配置
	config := &ssh.ClientConfig{
		Timeout:         time.Second, //ssh 连接time out 时间一秒钟, 如果ssh验证错误 会在一秒内返回
		User:            sshUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //这个可以， 但是不够安全
		//HostKeyCallback: hostKeyCallBackFunc(h.Host),
	}
	config.Auth = []ssh.AuthMethod{ssh.Password(sshPassword)}

	//dial 获取ssh client,支持远端的地址参数
	var addr string
	if sshAddr == "" {
		addr = fmt.Sprintf("%s:%d", kvmconf.Config.KVM.SSHHost, kvmconf.Config.KVM.SSHPort)
	} else {
		addr = sshAddr
	}
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	sshSession, err := sshClient.NewSession()
	if err != nil {
		return nil, err
	}

	dc.SendText("success")

	go sshHandler(sshClient, sshSession, dc, rtcin)

	return sshSession, nil
}

type Wrap struct {
	*webrtc.DataChannel
}

func (rtc *Wrap) Write(data []byte) (int, error) {
	err := rtc.DataChannel.Send(data)
	return len(data), err
}

func sshHandler(sshClient *ssh.Client, sshSession *ssh.Session, dc *webrtc.DataChannel, rtcin chan string) error {
	defer sshClient.Close()
	defer sshSession.Close()

	sshSession.Stdout = &Wrap{dc}
	sshSession.Stderr = &Wrap{dc}
	//sshSession.Stdin = &Wrap{dc}
	sshin, _ := sshSession.StdinPipe()
	go stdinHandler(sshin, rtcin)

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	// Request pseudo terminal
	if err := sshSession.RequestPty("xterm", 40, 80, modes); err != nil {
		return err
	}
	// Start remote shell
	if err := sshSession.Shell(); err != nil {
		return err
	}
	if err := sshSession.Wait(); err != nil {
		return err
	}

	return nil
}
func stdinHandler(sshin io.Writer, rtcin chan string) {
	for input := range rtcin {
		sshin.Write([]byte(input))
	}
}
