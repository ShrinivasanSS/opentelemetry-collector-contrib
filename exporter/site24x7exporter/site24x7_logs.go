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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
)

func (e *site24x7exporter) CreateLogItem(logrecord pdata.LogRecord, resourceAttr map[string]interface{}) (TelemetryLog) {
	startTime := (logrecord.Timestamp().AsTime().UnixNano() / int64(time.Millisecond))
	tlogBodyType := logrecord.Body().Type()
	tlogMsg := logrecord.Name()
	tlogTraceId := logrecord.TraceID().HexString()
	tlogSpanId := logrecord.SpanID().HexString()
	switch tlogBodyType {
	case pdata.AttributeValueTypeString:
		tlogMsg = logrecord.Body().AsString()
		io.WriteString(e.file, "\nLog Message from Body: \t" + tlogMsg)

	case pdata.AttributeValueTypeMap:
		tlogKvList := logrecord.Body().MapVal().AsRaw()
		// if kvlist gives "msg":"<logmsg>"
		tlogMsg = tlogKvList["msg"].(string)
		if len(tlogMsg) <= 0 {
			//tLogValue := v1.KeyValueList(tlogKvList).GetValues()
			tlogMsg = logrecord.Body().AsString()
		}
		tlogKvSpanId := tlogKvList["span_id"].(string)
		if len(tlogKvSpanId) > 0 {
			tlogSpanId = tlogKvSpanId
		}
		tlogKvTraceId := tlogKvList["trace_id"].(string)
		if len(tlogKvTraceId) > 0 {
			tlogTraceId = tlogKvTraceId
		}
	}

	tlog := TelemetryLog{
		Timestamp: startTime,
		S247UID: "otel-s247exporter",
		LogLevel: logrecord.SeverityText(),
		TraceId: tlogTraceId,
		SpanId: tlogSpanId,
		TraceFlag: logrecord.Flags(),
		ResourceAttributes: resourceAttr,
		LogAttributes: logrecord.Attributes().AsRaw(),
		Name: logrecord.Name(),
		Message: tlogMsg,
	}
	return tlog;
}

func (e *site24x7exporter) ConsumeLogs(_ context.Context, ld pdata.Logs) error {
	/*buf, err := logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}
	return exportMessageAsLine(e, buf)*/
	e.mutex.Lock()
	defer e.mutex.Unlock()

	var urlBuf bytes.Buffer
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
				logItem := e.CreateLogItem( rawLogitem, resourceAttr)
				logList = append(logList, logItem)
			}
		}
	}

	io.WriteString(e.file, "\nTransformed telemetry logs to site24x7 format. \n")
	buf, err := json.Marshal(logList)
	if err != nil{
		io.WriteString(e.file, "\nError in converting telemetry logs. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}
	responseBody := bytes.NewBuffer(buf)
	fmt.Fprint(&urlBuf, e.url, "?license.key=",e.apikey);
	resp, err := http.Post(urlBuf.String(), "application/json", responseBody)
	io.WriteString(e.file, "\nPosting telemetry logs to url. \n")
	if err != nil {
		io.WriteString(e.file, "\nError in posting logs to url. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if _, err := e.file.Write(body); err != nil {
		return err
	}
	return err
}
