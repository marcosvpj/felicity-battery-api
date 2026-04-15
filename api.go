package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const baseURL = "https://shine-api.felicitysolar.com"

type client struct {
	http     *http.Client
	username string
	password string
	token    string
	tokenAt  time.Time
}

func newClient(username, password string) *client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Felicity server uses self-signed cert
	}
	return &client{
		http:     &http.Client{Timeout: 15 * time.Second, Transport: transport},
		username: username,
		password: password,
	}
}

// --- Login ---

type loginResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

func (c *client) login() error {
	body, _ := json.Marshal(map[string]string{
		"userName": c.username,
		"password": c.password,
	})
	req, err := http.NewRequest("POST", baseURL+"/app/base/userlogin", bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.setHeaders(req, false)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	if lr.Code != 200 || lr.Data.Token == "" {
		return fmt.Errorf("login failed: %s (code %d)", lr.Message, lr.Code)
	}
	c.token = lr.Data.Token
	c.tokenAt = time.Now()
	return nil
}

func (c *client) ensureToken() error {
	if c.token == "" || time.Since(c.tokenAt) > 25*24*time.Hour {
		return c.login()
	}
	return nil
}

func (c *client) setHeaders(req *http.Request, auth bool) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("lang", "en_US")
	req.Header.Set("source", "WEB")
	if auth {
		req.Header.Set("Authorization", "Bearer_"+c.token)
	}
}

// --- Snapshot ---

// BatterySnapshot holds the real-time data returned by the API.
// Nullable fields use *string since many are null depending on device type.
type BatterySnapshot struct {
	DataTimeStr string  `json:"dataTimeStr"`
	DeviceSn    string  `json:"deviceSn"`
	DeviceModel *string `json:"deviceModel"`
	Alias       *string `json:"alias"`
	Status      *string `json:"status"`
	ReportFreq  int     `json:"reportFreq"`
	WifiSignal  *string `json:"wifiSignal"`

	// State
	BattSoc      *string `json:"battSoc"`
	BattSoh      *string `json:"battSoh"`
	BattCapacity *string `json:"battCapacity"`
	BmsState     *string `json:"bmsState"`

	// Electrical
	BattVolt *string `json:"battVolt"`
	BattCurr *string `json:"battCurr"`
	BmsPower *string `json:"bmsPower"`

	// Thermal
	TempMax *string `json:"tempMax"`
	TempMin *string `json:"tempMin"`

	// Cell voltages (mV as strings; "32767" = slot not populated)
	CellVolt1  *string `json:"cellVolt1"`
	CellVolt2  *string `json:"cellVolt2"`
	CellVolt3  *string `json:"cellVolt3"`
	CellVolt4  *string `json:"cellVolt4"`
	CellVolt5  *string `json:"cellVolt5"`
	CellVolt6  *string `json:"cellVolt6"`
	CellVolt7  *string `json:"cellVolt7"`
	CellVolt8  *string `json:"cellVolt8"`
	CellVolt9  *string `json:"cellVolt9"`
	CellVolt10 *string `json:"cellVolt10"`
	CellVolt11 *string `json:"cellVolt11"`
	CellVolt12 *string `json:"cellVolt12"`
	CellVolt13 *string `json:"cellVolt13"`
	CellVolt14 *string `json:"cellVolt14"`
	CellVolt15 *string `json:"cellVolt15"`
	CellVolt16 *string `json:"cellVolt16"`

	// Cell temperatures (°C)
	CellTemp1 *string `json:"cellTemp1"`
	CellTemp2 *string `json:"cellTemp2"`
	CellTemp3 *string `json:"cellTemp3"`
	CellTemp4 *string `json:"cellTemp4"`
}

func (s *BatterySnapshot) cellVolts() []*string {
	return []*string{
		s.CellVolt1, s.CellVolt2, s.CellVolt3, s.CellVolt4,
		s.CellVolt5, s.CellVolt6, s.CellVolt7, s.CellVolt8,
		s.CellVolt9, s.CellVolt10, s.CellVolt11, s.CellVolt12,
		s.CellVolt13, s.CellVolt14, s.CellVolt15, s.CellVolt16,
	}
}

func (s *BatterySnapshot) cellTemps() []*string {
	return []*string{s.CellTemp1, s.CellTemp2, s.CellTemp3, s.CellTemp4}
}

type snapshotResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    BatterySnapshot `json:"data"`
}

func (c *client) getSnapshot(deviceSn string) (*BatterySnapshot, error) {
	if err := c.ensureToken(); err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]string{
		"deviceSn":   deviceSn,
		"deviceType": "BP",
		"dateStr":    time.Now().Format("2006-01-02 15:04:05"),
	})
	req, err := http.NewRequest("POST", baseURL+"/device/get_device_snapshot", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, true)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("snapshot request: %w", err)
	}
	defer resp.Body.Close()

	var sr snapshotResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	if sr.Code != 200 {
		return nil, fmt.Errorf("API error: %s (code %d)", sr.Message, sr.Code)
	}
	return &sr.Data, nil
}
