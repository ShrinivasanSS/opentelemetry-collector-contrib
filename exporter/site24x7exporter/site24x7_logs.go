// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package site24x7exporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
)

func (e *site24x7exporter) CreateLogItem(logrecord pdata.LogRecord, resourceAttr map[string]interface{}) TelemetryLog {
	startTime := (logrecord.Timestamp().AsTime().UnixNano() / int64(time.Millisecond))
	tlogBodyType := logrecord.Body().Type()
	tlogMsg := logrecord.Name()
	tlogTraceId := logrecord.TraceID().HexString()
	tlogSpanId := logrecord.SpanID().HexString()
	switch tlogBodyType {
	case pdata.AttributeValueTypeString:
		tlogMsg = logrecord.Body().AsString()
		io.WriteString(e.file, "\nLog Message from Body: \t"+tlogMsg)

	case pdata.AttributeValueTypeMap:
		tlogKvList := logrecord.Body().MapVal().AsRaw()
		// if kvlist gives "msg":"<logmsg>"
		if attrVal, found := tlogKvList["msg"]; found {
			//tLogValue := v1.KeyValueList(tlogKvList).GetValues()
			tlogMsg = attrVal.(string)
		} else {
			tlogMsg = logrecord.Body().AsString()
		}

		if tlogKvSpanId, found := tlogKvList["span_id"]; found {
			tlogSpanId = tlogKvSpanId.(string)
		}

		if tlogKvTraceId, found := tlogKvList["trace_id"]; found {
			tlogTraceId = tlogKvTraceId.(string)
		}
	}

	tlog := TelemetryLog{
		Timestamp:          startTime,
		S247UID:            "otel-s247exporter",
		LogLevel:           logrecord.SeverityText(),
		TraceId:            tlogTraceId,
		SpanId:             tlogSpanId,
		TraceFlag:          logrecord.Flags(),
		ResourceAttributes: resourceAttr,
		LogAttributes:      logrecord.Attributes().AsRaw(),
		Name:               logrecord.Name(),
		Message:            tlogMsg,
	}
	return tlog
}

func (e *site24x7exporter) ConsumeLogs(_ context.Context, ld pdata.Logs) error {
	/*buf, err := logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}
	return exportMessageAsLine(e, buf)*/
	e.mutex.Lock()
	defer e.mutex.Unlock()

	logCount := ld.LogRecordCount()
	logList := make([]TelemetryLog, 0, logCount)
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		resource := rlogs.Resource()
		resourceAttr := resource.Attributes().AsRaw()
		instLogs := rlogs.InstrumentationLibraryLogs()
		for j := 0; j < instLogs.Len(); j++ {
			ilogs := instLogs.At(j)
			ilogItems := ilogs.Logs()
			for k := 0; k < ilogItems.Len(); k++ {
				rawLogitem := ilogItems.At(k)
				logItem := e.CreateLogItem(rawLogitem, resourceAttr)
				logList = append(logList, logItem)
			}
		}
	}

	io.WriteString(e.file, "\nTransformed telemetry logs to site24x7 format. \n")
	buf, err := json.Marshal(logList)
	if err != nil {
		io.WriteString(e.file, "\nError in converting telemetry logs. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}
	
	/*
	var urlBuf bytes.Buffer
	responseBody := bytes.NewBuffer(buf)
	fmt.Fprint(&urlBuf, e.url, "?license.key=", e.apikey)
	fmt.Println("Sending to Site24x7: ", responseBody)
	resp, err := http.Post(urlBuf.String(), "application/json", responseBody)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if _, err := e.file.Write(body); err != nil {
		return err
	}
	*/
	/*
		var gzipbuf bytes.Buffer
		gzipRespBody := gzip.NewWriter(&gzipbuf)
		if _, err := gzipRespBody.Write(&gzipbuf); err != nil {
			io.WriteString(e.file, "\n Compressing to GZIP failed. \n")
			return err
		}
		if err := g.close(); err != nil {
			io.WriteString(e.file, "\n Closing GZIP buffere failed\n")
			return err
		}
		resp, err := http.Post(urlBuf.String(), "application/json", &gzipbuf)
	*/

	client := http.Client{}

	var gzbuf bytes.Buffer
	g := gzip.NewWriter(&gzbuf)
	g.Write(buf)
	g.Close()
	req , err := http.NewRequest("POST", e.url, &gzbuf)
	if err != nil {
		//Handle Error
		fmt.Println("Error initializing Url: ", err)
		io.WriteString(e.file, "\nError in posting logs to url. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}

	req.Header = http.Header{
		"X-DeviceKey": []string{e.apikey},
		"Content-Type": []string{"application/json"},
		"X-LogType": []string{"otellogs"},
		"X-StreamMode": []string{"1"},
		"Log-Size": []string{strconv.Itoa(len(logList))},
		"Content-Encoding": []string{"gzip"},
		"User-Agent": []string{"AWS-Lambda"},
	}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: e.insecure}
	res , err := client.Do(req)
	if err != nil {
		//Handle Error
		fmt.Println("Error initializing Url: ", err)
		io.WriteString(e.file, "\nError in posting logs to url. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}
	io.WriteString(e.file, "\nPosting telemetry logs to url. \n")
	uploadid := res.Header.Values("x-uploadid")
	io.WriteString(e.file, "Upload ID: " + strings.Join(uploadid," "))
	fmt.Println("Uploaded logs information: ", res.Header)
	if err != nil {
		io.WriteString(e.file, "\nError in posting logs to url. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}
	
	return err
}
