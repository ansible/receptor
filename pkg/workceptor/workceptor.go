//go:build !no_workceptor
// +build !no_workceptor

package workceptor

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ansible/receptor/pkg/certificates"
	"github.com/ansible/receptor/pkg/controlsvc"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
	"github.com/ansible/receptor/pkg/randstr"
	"github.com/ansible/receptor/pkg/utils"
	"github.com/golang-jwt/jwt/v4"
)

// NetceptorForWorkceptor is a interface to decouple workceptor from netceptor.
// it includes only the functions that workceptor uses.
type NetceptorForWorkceptor interface {
	NodeID() string
	AddWorkCommand(typeName string, verifySignature bool) error
	GetClientTLSConfig(name string, expectedHostName string, expectedHostNameType netceptor.ExpectedHostnameType) (*tls.Config, error) // have a common pkg for types
	GetLogger() *logger.ReceptorLogger
	DialContext(ctx context.Context, node string, service string, tlscfg *tls.Config) (*netceptor.Conn, error) // create an interface for Conn
}

type ServerForWorkceptor interface {
	AddControlFunc(name string, cType controlsvc.ControlCommandType) error
	ConnectionListener(ctx context.Context, listener net.Listener)
	RunControlSession(conn net.Conn)
	RunControlSvc(ctx context.Context, service string, tlscfg *tls.Config, unixSocket string, unixSocketPermissions fs.FileMode, tcpListen string, tcptls *tls.Config) error
	SetServerNet(n controlsvc.Neter)
	SetServerTLS(t controlsvc.Tlser)
	SetServerUtils(u controlsvc.Utiler)
	SetupConnection(conn net.Conn)
}

// Workceptor is the main object that handles unit-of-work management.
type Workceptor struct {
	ctx               context.Context
	Cancel            context.CancelFunc
	nc                NetceptorForWorkceptor
	dataDir           string
	workTypesLock     *sync.RWMutex
	workTypes         map[string]*workType
	activeUnitsLock   *sync.RWMutex
	activeUnits       map[string]WorkUnit
	SigningKey        string
	SigningExpiration time.Duration
	VerifyingKey      string
}

// workType is the record for a registered type of work.
type workType struct {
	newWorkerFunc   NewWorkerFunc
	verifySignature bool
}

// New constructs a new Workceptor instance.
func New(ctx context.Context, nc NetceptorForWorkceptor, dataDir string) (*Workceptor, error) {
	dataDir = setDataDir(dataDir, nc)
	c, cancel := context.WithCancel(ctx)
	w := &Workceptor{
		ctx:               c,
		Cancel:            cancel,
		nc:                nc,
		dataDir:           dataDir,
		workTypesLock:     &sync.RWMutex{},
		workTypes:         make(map[string]*workType),
		activeUnitsLock:   &sync.RWMutex{},
		activeUnits:       make(map[string]WorkUnit),
		SigningKey:        "",
		SigningExpiration: 5 * time.Minute,
		VerifyingKey:      "",
	}
	err := w.RegisterWorker("remote", newRemoteWorker, false)
	if err != nil {
		return nil, fmt.Errorf("could not register remote worker function: %s", err)
	}

	return w, nil
}

// MainInstance is the global instance of Workceptor instantiated by the command-line main() function.
var MainInstance *Workceptor

// setDataDir returns a valid data directory.
func setDataDir(dataDir string, nc NetceptorForWorkceptor) string {
	_, err := os.Stat(dataDir)
	if err == nil {
		return path.Join(dataDir, nc.NodeID())
	}
	nc.GetLogger().Warning("Receptor data directory provided does not exist \"%s\". Trying the default '/var/lib/receptor/", dataDir)

	dataDir = "/var/lib/receptor"
	_, err = os.Stat(dataDir)
	if err == nil {
		return path.Join(dataDir, nc.NodeID())
	}
	nc.GetLogger().Warning("Receptor data directory \"%s\" does not exist. Setting tmp '/tmp/receptor/", dataDir)

	dataDir = path.Join(os.TempDir(), "receptor")
	dataDir = path.Join(dataDir, nc.NodeID())

	return dataDir
}

// stdoutSize returns size of stdout, if it exists, or 0 otherwise.
func stdoutSize(unitdir string) int64 {
	stat, err := os.Stat(path.Join(unitdir, "stdout"))
	if err != nil {
		return 0
	}

	return stat.Size()
}

// RegisterWithControlService registers this workceptor instance with a control service instance.
func (w *Workceptor) RegisterWithControlService(cs ServerForWorkceptor) error {
	err := cs.AddControlFunc("work", &workceptorCommandType{
		w: w,
	})
	if err != nil {
		return fmt.Errorf("could not add work control function: %s", err)
	}

	return nil
}

// RegisterWorker notifies the Workceptor of a new kind of work that can be done.
func (w *Workceptor) RegisterWorker(typeName string, newWorkerFunc NewWorkerFunc, verifySignature bool) error {
	w.workTypesLock.Lock()
	_, ok := w.workTypes[typeName]
	if ok {
		w.workTypesLock.Unlock()

		return fmt.Errorf("work type %s already registered", typeName)
	}
	w.workTypes[typeName] = &workType{
		newWorkerFunc:   newWorkerFunc,
		verifySignature: verifySignature,
	}
	if typeName != "remote" { // all workceptors get a remote command by default
		w.nc.AddWorkCommand(typeName, verifySignature)
	}
	w.workTypesLock.Unlock()

	// Check if any unknown units have now become known
	w.activeUnitsLock.Lock()
	for id, worker := range w.activeUnits {
		_, ok := worker.(*unknownUnit)
		if ok && worker.Status().WorkType == typeName {
			delete(w.activeUnits, id)
		}
	}
	w.activeUnitsLock.Unlock()
	w.scanForUnits()

	return nil
}

func (w *Workceptor) generateUnitID(lock bool) (string, error) {
	if lock {
		w.activeUnitsLock.RLock()
		defer w.activeUnitsLock.RUnlock()
	}
	var ident string
	for {
		ident = randstr.RandomString(8)
		_, ok := w.activeUnits[ident]
		if !ok {
			unitdir := path.Join(w.dataDir, ident)
			_, err := os.Stat(unitdir)
			if err == nil {
				continue
			}

			return ident, os.MkdirAll(unitdir, 0o700)
		}
	}
}

func (w *Workceptor) createSignature(nodeID string) (string, error) {
	if w.SigningKey == "" {
		return "", fmt.Errorf("cannot sign work: signing key is empty")
	}
	exp := time.Now().Add(w.SigningExpiration)

	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(exp),
		Audience:  []string{nodeID},
	}
	rsaPrivateKey, err := certificates.LoadPrivateKey(w.SigningKey, &certificates.OsWrapper{})
	if err != nil {
		return "", fmt.Errorf("could not load signing key file: %s", err.Error())
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	tokenString, err := token.SignedString(rsaPrivateKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (w *Workceptor) ShouldVerifySignature(workType string, signWork bool) bool {
	// if work unit is remote, just get the signWork boolean from the
	// remote extra data field
	if workType == "remote" {
		return signWork
	}
	w.workTypesLock.RLock()
	wt, ok := w.workTypes[workType]
	w.workTypesLock.RUnlock()
	if ok && wt.verifySignature {
		return true
	}

	return false
}

func (w *Workceptor) VerifySignature(signature string) error {
	if signature == "" {
		return fmt.Errorf("could not verify signature: signature is empty")
	}
	if w.VerifyingKey == "" {
		return fmt.Errorf("could not verify signature: verifying key not specified")
	}
	rsaPublicKey, err := certificates.LoadPublicKey(w.VerifyingKey, &certificates.OsWrapper{})
	if err != nil {
		return fmt.Errorf("could not load verifying key file: %s", err.Error())
	}
	token, err := jwt.ParseWithClaims(signature, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return rsaPublicKey, nil
	})
	if err != nil {
		return fmt.Errorf("could not verify signature: %s", err.Error())
	}
	if !token.Valid {
		return fmt.Errorf("token not valid")
	}
	claims := token.Claims.(*jwt.RegisteredClaims)
	ok := claims.VerifyAudience(w.nc.NodeID(), true)
	if !ok {
		return fmt.Errorf("token audience did not match node ID")
	}

	return nil
}

// AllocateUnit creates a new local work unit and generates an identifier for it.
func (w *Workceptor) AllocateUnit(workTypeName string, params map[string]string) (WorkUnit, error) {
	w.workTypesLock.RLock()
	wt, ok := w.workTypes[workTypeName]
	w.workTypesLock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown work type %s", workTypeName)
	}
	w.activeUnitsLock.Lock()
	defer w.activeUnitsLock.Unlock()
	ident, err := w.generateUnitID(false)
	if err != nil {
		return nil, err
	}
	worker := wt.newWorkerFunc(nil, w, ident, workTypeName)
	err = worker.SetFromParams(params)
	if err == nil {
		err = worker.Save()
	}
	if err != nil {
		return nil, err
	}
	w.activeUnits[ident] = worker

	return worker, nil
}

// AllocateRemoteUnit creates a new remote work unit and generates a local identifier for it.
func (w *Workceptor) AllocateRemoteUnit(remoteNode, remoteWorkType, tlsClient, ttl string, signWork bool, params map[string]string) (WorkUnit, error) {
	if tlsClient != "" {
		_, err := w.nc.GetClientTLSConfig(tlsClient, "testhost", netceptor.ExpectedHostnameTypeReceptor)
		if err != nil {
			return nil, err
		}
	}
	hasSecrets := false
	for k := range params {
		if strings.HasPrefix(strings.ToLower(k), "secret_") {
			hasSecrets = true

			break
		}
	}
	if hasSecrets && tlsClient == "" {
		return nil, fmt.Errorf("cannot send secrets over a non-TLS connection")
	}
	rw, err := w.AllocateUnit("remote", params)
	if err != nil {
		return nil, err
	}
	var expiration time.Time
	if ttl != "" {
		duration, err := time.ParseDuration(ttl)
		if err != nil {
			w.nc.GetLogger().Error("Failed to parse provided ttl -- valid examples include '1.5h', '30m', '30m10s'")

			return nil, err
		}
		if signWork && duration > w.SigningExpiration {
			w.nc.GetLogger().Warning("json web token expires before ttl")
		}
		expiration = time.Now().Add(duration)
	} else {
		expiration = time.Time{}
	}
	rw.UpdateFullStatus(func(status *StatusFileData) {
		ed := status.ExtraData.(*RemoteExtraData)
		ed.RemoteNode = remoteNode
		ed.RemoteWorkType = remoteWorkType
		ed.TLSClient = tlsClient
		ed.Expiration = expiration
		ed.SignWork = signWork
	})
	if rw.LastUpdateError() != nil {
		return nil, rw.LastUpdateError()
	}

	return rw, nil
}

func (w *Workceptor) scanForUnit(unitID string) {
	unitdir := path.Join(w.dataDir, unitID)
	fi, _ := os.Stat(unitdir)
	if fi == nil || !fi.IsDir() {
		w.nc.GetLogger().Error("Error locating unit: %s", unitID)

		return
	}
	ident := fi.Name()
	w.activeUnitsLock.RLock()
	_, ok := w.activeUnits[ident] //nolint:ifshort
	w.activeUnitsLock.RUnlock()
	if !ok {
		statusFilename := path.Join(unitdir, "status")
		sfd := &StatusFileData{}
		_ = sfd.Load(statusFilename)
		w.workTypesLock.RLock()
		wt, ok := w.workTypes[sfd.WorkType]
		w.workTypesLock.RUnlock()
		var worker WorkUnit
		if ok {
			worker = wt.newWorkerFunc(nil, w, ident, sfd.WorkType)
		} else {
			worker = newUnknownWorker(w, ident, sfd.WorkType)
		}
		if _, err := os.Stat(statusFilename); os.IsNotExist(err) {
			w.nc.GetLogger().Error("Status file has disappeared for %s.", ident)

			return
		}
		err := worker.Load()
		if err != nil {
			w.nc.GetLogger().Warning("Failed to restart worker %s due to read error: %s", unitdir, err)
			worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Failed to restart: %s", err), stdoutSize(unitdir))
		}
		err = worker.Restart()
		if err != nil && !IsPending(err) {
			w.nc.GetLogger().Warning("Failed to restart worker %s: %s", unitdir, err)
			worker.UpdateBasicStatus(WorkStateFailed, fmt.Sprintf("Failed to restart: %s", err), stdoutSize(unitdir))
		}
		w.activeUnitsLock.Lock()
		defer w.activeUnitsLock.Unlock()
		w.activeUnits[ident] = worker
	}
}

func (w *Workceptor) scanForUnits() {
	files, err := os.ReadDir(w.dataDir)
	if err != nil {
		return
	}
	for i := range files {
		fi := files[i]
		w.scanForUnit(fi.Name())
	}
}

func (w *Workceptor) findUnit(unitID string) (WorkUnit, error) {
	w.activeUnitsLock.RLock()
	defer w.activeUnitsLock.RUnlock()
	unit, ok := w.activeUnits[unitID]
	if ok {
		return unit, nil
	}
	// if not in active units, rescan work unit dir and recheck
	w.scanForUnit(unitID)
	unit, ok = w.activeUnits[unitID]
	if !ok {
		return nil, fmt.Errorf("unknown work unit %s", unitID)
	}

	return unit, nil
}

// StartUnit starts a unit of work.
func (w *Workceptor) StartUnit(unitID string) error {
	unit, err := w.findUnit(unitID)
	if err != nil {
		return err
	}

	return unit.Start()
}

// ListKnownUnitIDs returns a slice containing the known unit IDs.
func (w *Workceptor) ListKnownUnitIDs() []string {
	w.activeUnitsLock.RLock()
	defer w.activeUnitsLock.RUnlock()
	result := make([]string, 0, len(w.activeUnits))
	for id := range w.activeUnits {
		result = append(result, id)
	}

	return result
}

// UnitStatus returns the state of a unit.
func (w *Workceptor) UnitStatus(unitID string) (*StatusFileData, error) {
	unit, err := w.findUnit(unitID)
	if err != nil {
		return nil, err
	}

	return unit.Status(), nil
}

// CancelUnit cancels a unit of work, killing any processes.
func (w *Workceptor) CancelUnit(unitID string) error {
	unit, err := w.findUnit(unitID)
	if err != nil {
		return err
	}

	return unit.Cancel()
}

// ReleaseUnit releases (deletes) resources from a unit of work, including stdout.  Release implies Cancel.
func (w *Workceptor) ReleaseUnit(unitID string, force bool) error {
	unit, err := w.findUnit(unitID)
	if err != nil {
		return err
	}

	return unit.Release(force)
}

// unitStatusForCFR returns status information as a map, suitable for a control function return value.
func (w *Workceptor) unitStatusForCFR(unitID string) (map[string]interface{}, error) {
	status, err := w.UnitStatus(unitID)
	if err != nil {
		return nil, err
	}
	retMap := make(map[string]interface{})
	v := reflect.ValueOf(*status)
	t := reflect.TypeOf(*status)
	for i := 0; i < v.NumField(); i++ {
		retMap[t.Field(i).Name] = v.Field(i).Interface()
	}
	retMap["StateName"] = WorkStateToString(status.State)

	return retMap, nil
}

// sleepOrDone sleeps until a timeout or the done channel is signaled.
func sleepOrDone(doneChan <-chan struct{}, interval time.Duration) bool {
	select {
	case <-doneChan:
		return true
	case <-time.After(interval):
		return false
	}
}

// GetResults returns a live stream of the results of a unit.
func (w *Workceptor) GetResults(ctx context.Context, unitID string, startPos int64) (chan []byte, error) {
	unit, err := w.findUnit(unitID)
	if err != nil {
		return nil, err
	}
	resultChan := make(chan []byte)
	closeOnce := sync.Once{}
	resultClose := func() {
		closeOnce.Do(func() {
			close(resultChan)
		})
	}
	unitdir := path.Join(w.dataDir, unitID)
	stdoutFilename := path.Join(unitdir, "stdout")
	var stdout *os.File
	ctxChild, cancel := context.WithCancel(ctx)
	go func() {
		defer func() {
			err = stdout.Close()
			if err != nil {
				w.nc.GetLogger().Error("Error closing stdout %s", stdoutFilename)
			}
			resultClose()
			cancel()
		}()

		// Wait for stdout file to exist
		for {
			stdout, err = os.Open(stdoutFilename)
			switch {
			case err == nil:
			case os.IsNotExist(err):
				if IsComplete(unit.Status().State) {
					w.nc.GetLogger().Warning("Unit completed without producing any stdout\n")

					return
				}
				if sleepOrDone(ctx.Done(), 500*time.Millisecond) {
					return
				}

				continue
			default:
				w.nc.GetLogger().Error("Error accessing stdout file: %s\n", err)

				return
			}

			break
		}
		filePos := startPos
		statChan := make(chan struct{}, 1)
		go func() {
			failures := 0
			for {
				select {
				case <-ctxChild.Done():
					return
				case <-time.After(1 * time.Second):
					_, err := os.Stat(stdoutFilename)
					if os.IsNotExist(err) {
						failures++
						if failures > 3 {
							w.nc.GetLogger().Error("Exceeded retries for reading stdout %s", stdoutFilename)
							statChan <- struct{}{}

							return
						}
					} else {
						failures = 0
					}
				}
			}
		}()
		for {
			if sleepOrDone(ctx.Done(), 250*time.Millisecond) {
				return
			}
			for {
				select {
				case <-ctx.Done():
					return
				case <-statChan:
					return
				default:
					var newPos int64
					newPos, err = stdout.Seek(filePos, 0)
					if err != nil {
						w.nc.GetLogger().Warning("Seek error processing stdout: %s\n", err)

						return
					}
					if newPos != filePos {
						w.nc.GetLogger().Warning("Seek error processing stdout\n")

						return
					}
					var n int
					buf := make([]byte, utils.NormalBufferSize)
					n, err = stdout.Read(buf)
					if n > 0 {
						filePos += int64(n)
						select {
						case <-ctx.Done():
							return
						case resultChan <- buf[:n]:
						}
					}
				}
				if err != nil {
					break
				}
			}
			if err == io.EOF {
				unitStatus := unit.Status()
				if IsComplete(unitStatus.State) && filePos >= unitStatus.StdoutSize {
					w.nc.GetLogger().Debug("Stdout complete - closing channel for: %s \n", unitID)

					return
				}
			} else if err != nil {
				w.nc.GetLogger().Error("Error reading stdout: %s\n", err)

				return
			}
		}
	}()

	return resultChan, nil
}
