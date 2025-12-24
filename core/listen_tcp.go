package core

import (
	"bastion/models"
	"fmt"
	"net"
	"strconv"
)

func listenTCPWithDiagnostics(mapping *models.Mapping) (net.Listener, error) {
	addr := net.JoinHostPort(mapping.LocalHost, strconv.Itoa(mapping.LocalPort))
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		return listener, nil
	}

	if !isAddrInUse(err) {
		return nil, err
	}

	detail := DiagnosePortInUse("tcp", mapping.LocalHost, mapping.LocalPort)
	detail.ListenError = err.Error()

	return nil, &PortInUseError{
		Detail: detail,
		Cause:  NewResourceBusyError(fmt.Sprintf("Port %d is already in use", mapping.LocalPort)),
	}
}
