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


func (e *site24x7exporter) CreateTelemetrySpan(span pdata.Span, 
	resourceAttr map[string]interface{}, 
	serviceName string,
	instLibrary string, 
	instLibraryVersion string,
	telSDKLang string, 
	telSDKName string) (TelemetrySpan) {
	
	spanAttr := span.Attributes().AsRaw()
	startTime := (span.StartTimestamp().AsTime().UnixNano()) // int64(time.Millisecond))
	endTime := (span.EndTimestamp().AsTime().UnixNano()) // int64(time.Millisecond))
	spanEvts := span.Events()
	telEvents := make([]TelemetrySpanEvent,0,spanEvts.Len())
	exceptionMessages := make([]string, 0, spanEvts.Len())
	exceptionStackTraces := make([]string, 0, spanEvts.Len())
	exceptionTypes := make([]string, 0, spanEvts.Len())
	for i := 0; i < spanEvts.Len(); i++ {
		spanEvt := spanEvts.At(i)
		telEvt := TelemetrySpanEvent{
			Timestamp: (spanEvt.Timestamp().AsTime().UnixNano() / int64(time.Millisecond)),
			Name: spanEvt.Name(),
			EventAttributes: spanEvt.Attributes().AsRaw(),
		}
		exMsg,found := telEvt.EventAttributes["exception.message"]
		if found {
			exceptionMessages = append(exceptionMessages, exMsg.(string))
		}

		exST, found := telEvt.EventAttributes["exception.stacktrace"]
		if found {
			exceptionStackTraces = append(exceptionStackTraces, exST.(string))
		}
		exType, found := telEvt.EventAttributes["exception.type"]
		if found {
			exceptionTypes = append(exceptionTypes, exType.(string))
		}
		telEvents = append(telEvents, telEvt)
	}
	spanLinks := span.Links()
	telLinks := make([]TelemetrySpanLink, 0, spanLinks.Len())
	for i := 0; i < spanLinks.Len(); i++ {
		spanLink := spanLinks.At(i)
		telLink := TelemetrySpanLink{
			LinkSpanID: spanLink.SpanID().HexString(),
			LinkTraceID: spanLink.TraceID().HexString(),
		}
		telLinks = append(telLinks, telLink)
	}
	spanState := span.Status()

	// Host attributes
	var hostIp, hostName, hostPort string
	if attrval,found := spanAttr["net.peer.ip"]; found {
		hostIp = attrval.(string)
	}
	if attrval,found := spanAttr["net.peer.name"]; found {
		hostName = attrval.(string)
	}
	if attrval,found := spanAttr["net.peer.port"]; found {
		hostPort = attrval.(string)
	}
	// Thread attributes
	var threadid, threadname string
	if attrval,found := spanAttr["thread.id"]; found {
		threadid = attrval.(string)
	}
	if attrval,found := spanAttr["thread.name"]; found {
		threadname = attrval.(string)
	}
	// DB attributes
	var dbsystem, dbstmt, dbname, dbconnstr string
	if attrval,found := spanAttr["db.system"]; found {
		dbsystem = attrval.(string)
	}
	if attrval,found := spanAttr["db.statement"]; found {
		dbstmt = attrval.(string)
	}
	if attrval,found := spanAttr["db.name"]; found {
		dbname = attrval.(string)
	}
	if attrval,found := spanAttr["db.connection_string"]; found {
		dbconnstr = attrval.(string)
	}
	// Http attributes
	var httpurl, httpmethod, httpstatus string
	if attrval,found := spanAttr["http.url"]; found {
		httpurl = attrval.(string)
	}
	if attrval,found := spanAttr["http.method"]; found {
		httpmethod = attrval.(string)
	}
	if attrval,found := spanAttr["http.status_code"]; found {
		httpstatus = attrval.(string)
	}

	isRoot := span.ParentSpanID().IsEmpty()
	
	tspan := TelemetrySpan{
		SpanId: span.SpanID().HexString(),
		TraceId: span.TraceID().HexString(),
		ParentSpanId: span.ParentSpanID().HexString(),
		Name: span.Name(),
		Kind: span.Kind().String(),
		StartTime: startTime,
		EndTime: endTime,
		Duration: (endTime - startTime),
		ResourceAttributes: resourceAttr,
		SpanAttributes: spanAttr,
		ServiceName: serviceName,
		Events: telEvents,
		Links: telLinks,
		StatusCode: spanState.Code().String(),
		StatusMsg: spanState.Message(),
		DroppedAttributesCount: span.DroppedAttributesCount(),
		DroppedLinksCount: span.DroppedLinksCount(),
		DroppedEventsCount: span.DroppedEventsCount(),
		TraceState: string(span.TraceState()),

		ExceptionMessage: exceptionMessages,
		ExceptionStackTrace: exceptionStackTraces,
		ExceptionType: exceptionTypes,

		InstrumentationLibrary: instLibrary,
		InstrumentationLibraryVersion: instLibraryVersion,
		TelemetrySDKLanguage: telSDKLang,
		TelemetrySDKName: telSDKName,

		HostIP: hostIp,
		HostName: hostName,
		HostPort: hostPort,

		ThreadId: threadid,
		ThreadName: threadname,

		DbSystem: dbsystem,
		DbStatement: dbstmt,
		DbName: dbname,
		DbConnStr: dbconnstr,

		HttpUrl: httpurl,
		HttpMethod: httpmethod,
		HttpStatusCode: httpstatus,

		IsRoot: isRoot,

	}
	return tspan;
}


func (e *site24x7exporter) ConsumeTraces(_ context.Context, td pdata.Traces) error {
	/*buf, err := tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	return exportMessageAsLine(e, buf)*/

	e.mutex.Lock()
	defer e.mutex.Unlock()

	var urlBuf bytes.Buffer
	resourcespans := td.ResourceSpans()
	spanCount := td.SpanCount()

	spanList := make([]TelemetrySpan, 0, spanCount)
	for i := 0; i < resourcespans.Len(); i++ {
		rspans := resourcespans.At(i)
		resource := rspans.Resource()
		resourceAttr := resource.Attributes().AsRaw()
		serviceName := resourceAttr["service.name"].(string)
		telSDKName := resourceAttr["telemetry.sdk.name"].(string)
		telSDKLang := resourceAttr["telemetry.sdk.language"].(string)
		instSpans := rspans.InstrumentationLibrarySpans()

		for j := 0; j < instSpans.Len(); j++ {
			ispans := instSpans.At(j)
			instLibName := ispans.InstrumentationLibrary().Name()
			instLibVer := ispans.InstrumentationLibrary().Version()
			ispanItems := ispans.Spans()
			for k := 0; k < ispanItems.Len(); k++ {
				span := ispanItems.At(k)
				s247span := e.CreateTelemetrySpan(span, resourceAttr, 
					serviceName, 
					instLibName, instLibVer,
					telSDKLang, telSDKName)
				spanList = append(spanList, s247span)
			}
		}
	}
	io.WriteString(e.file, "\nTransformed telemetry data to site24x7 format. \n")
	buf, err := json.Marshal(spanList)
	if err != nil{
		io.WriteString(e.file, "\nError in converting telemetry data. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}
	responseBody := bytes.NewBuffer(buf)
	fmt.Fprint(&urlBuf, e.url, "?license.key=",e.apikey);
	resp, err := http.Post(urlBuf.String(), "application/json", responseBody)
	io.WriteString(e.file, "\nPosting telemetry data to url. \n")
	if err != nil {
		io.WriteString(e.file, "\nError in posting data to url. \n")
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
