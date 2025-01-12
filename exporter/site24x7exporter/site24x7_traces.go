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
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
)

func (e *site24x7exporter) CreateTelemetrySpan(span pdata.Span,
	resourceAttr map[string]interface{},
	serviceName string,
	instLibrary string,
	instLibraryVersion string,
	telSDKLang string,
	telSDKName string,
	rootSpanId string,) TelemetrySpan {

	spanAttr := span.Attributes().AsRaw()
	startTime := (span.StartTimestamp().AsTime().UnixNano()) // int64(time.Millisecond))
	endTime := (span.EndTimestamp().AsTime().UnixNano())     // int64(time.Millisecond))
	spanEvts := span.Events()
	telEvents := make([]TelemetrySpanEvent, 0, spanEvts.Len())
	exceptionMessages := make([]string, 0, spanEvts.Len())
	exceptionStackTraces := make([]string, 0, spanEvts.Len())
	exceptionTypes := make([]string, 0, spanEvts.Len())
	for i := 0; i < spanEvts.Len(); i++ {
		spanEvt := spanEvts.At(i)
		telEvt := TelemetrySpanEvent{
			Timestamp:       (spanEvt.Timestamp().AsTime().UnixNano() / int64(time.Millisecond)),
			Name:            spanEvt.Name(),
			EventAttributes: spanEvt.Attributes().AsRaw(),
		}
		if telEvt.EventAttributes != nil {

			if exMsg, found := telEvt.EventAttributes["exception.message"]; found {
				exceptionMessages = append(exceptionMessages, exMsg.(string))
			}

			if exST, found := telEvt.EventAttributes["exception.stacktrace"]; found {
				exceptionStackTraces = append(exceptionStackTraces, exST.(string))
			}

			if exType, found := telEvt.EventAttributes["exception.type"]; found {
				exceptionTypes = append(exceptionTypes, exType.(string))
			}
		}
		telEvents = append(telEvents, telEvt)
	}
	spanLinks := span.Links()
	telLinks := make([]TelemetrySpanLink, 0, spanLinks.Len())
	for i := 0; i < spanLinks.Len(); i++ {
		spanLink := spanLinks.At(i)
		telLink := TelemetrySpanLink{
			LinkSpanID:  spanLink.SpanID().HexString(),
			LinkTraceID: spanLink.TraceID().HexString(),
		}
		telLinks = append(telLinks, telLink)
	}
	spanState := span.Status()
	spanStatus := spanState.Code().String()
	hasError := false
	switch spanStatus {
	case "STATUS_CODE_ERROR":
		hasError = true
	}

	spanKind := "UNSPECIFIED"
	switch span.Kind() {
	case pdata.SpanKindInternal:
		spanKind = "INTERNAL"
	case pdata.SpanKindServer:
		spanKind = "SERVER"
	case pdata.SpanKindClient:
		spanKind = "CLIENT"
	case pdata.SpanKindProducer:
		spanKind = "PRODUCER"
	case pdata.SpanKindConsumer:
		spanKind = "CONSUMER"
	}

	// Host attributes
	var hostIp, hostName, threadname string
	var hostPort, threadid int64
	if attrval, found := spanAttr["net.peer.ip"]; found {
		hostIp = attrval.(string)
	}
	if attrval, found := spanAttr["net.peer.name"]; found {
		hostName = attrval.(string)
	}
	if attrval, found := spanAttr["net.peer.port"]; found {
		hostPort = attrval.(int64)
	}
	// Thread attributes
	if attrval, found := spanAttr["thread.id"]; found {
		threadid = attrval.(int64)
	}
	if attrval, found := spanAttr["thread.name"]; found {
		threadname = attrval.(string)
	}
	// DB attributes
	var dbsystem, dbstmt, dbname, dbconnstr string
	if attrval, found := spanAttr["db.system"]; found {
		dbsystem = attrval.(string)
	}
	if attrval, found := spanAttr["db.statement"]; found {
		dbstmt = attrval.(string)
	}
	if attrval, found := spanAttr["db.name"]; found {
		dbname = attrval.(string)
	}
	if attrval, found := spanAttr["db.connection_string"]; found {
		dbconnstr = attrval.(string)
	}
	// Http attributes
	var httpurl, httpmethod string
	var httpstatus int64
	if attrval, found := spanAttr["http.url"]; found {
		httpurl = attrval.(string)
	}
	if attrval, found := spanAttr["http.method"]; found {
		httpmethod = attrval.(string)
	}
	if attrval, found := spanAttr["http.status_code"]; found {
		httpstatus = attrval.(int64)
	}

	isRoot := span.ParentSpanID().IsEmpty()
	telemetryParams := make([]TelemetryCustomParam, 0, len(spanAttr))
	for k, v := range spanAttr {
		telemetrycustomParam := TelemetryCustomParam{
			Key:   k,
			Value: v,
		}
		telemetryParams = append(telemetryParams, telemetrycustomParam)
	}

	spanid := span.SpanID().HexString()
	traceId := span.TraceID().HexString()
	parentspanId := span.ParentSpanID().HexString()
	spanName := span.Name()
	
	//fmt.Println("Creating telemetry span: Trace/Span/Parent/Root:  ", traceId," / ", spanid," / ", parentspanId," / ", rootSpanId)
	startTimeMs := (startTime / int64(time.Millisecond))

	tspan := TelemetrySpan{
		Timestamp:          	startTimeMs,
		S247UID:	            "otel-s247exporter",
		SpanId:                 spanid,
		TraceId:                traceId,
		ParentSpanId:           parentspanId,
		RootSpanId: 			rootSpanId,
		Name:                   spanName,
		Kind:                   spanKind,
		StartTime:              startTime,
		EndTime:                endTime,
		Duration:               float64(endTime-startTime) / float64(time.Millisecond),
		ServiceName:            serviceName,
		resourceAttributes:     resourceAttr,
		spanAttributes:         spanAttr,
		traceState:             string(span.TraceState()),
		spanEvents:             telEvents,
		spanLinks:              telLinks,
		statusCode:             spanState.Code().String(),
		statusMsg:              spanState.Message(),
		droppedAttributesCount: span.DroppedAttributesCount(),
		droppedLinksCount:      span.DroppedLinksCount(),
		droppedEventsCount:     span.DroppedEventsCount(),

		ExceptionMessage:    exceptionMessages,
		ExceptionStackTrace: exceptionStackTraces,
		ExceptionType:       exceptionTypes,

		InstrumentationLibrary:        instLibrary,
		InstrumentationLibraryVersion: instLibraryVersion,
		TelemetrySDKLanguage:          telSDKLang,
		TelemetrySDKName:              telSDKName,

		HostIP:   hostIp,
		HostName: hostName,
		HostPort: hostPort,

		ThreadId:   threadid,
		ThreadName: threadname,

		DbSystem:    dbsystem,
		DbStatement: dbstmt,
		DbName:      dbname,
		DbConnStr:   dbconnstr,

		HttpUrl:        httpurl,
		HttpMethod:     httpmethod,
		HttpStatusCode: httpstatus,

		IsRoot:   isRoot,
		HasError: hasError,

		CustomParams: telemetryParams,
	}
	return tspan
}

func (e *site24x7exporter) ConsumeTraces(_ context.Context, td pdata.Traces) error {
	/*buf, err := tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	return exportMessageAsLine(e, buf)*/

	e.mutex.Lock()
	defer e.mutex.Unlock()

	resourcespans := td.ResourceSpans()
	spanCount := td.SpanCount()
	rootSpanList := make(map[string]string)

	spanList := make([]TelemetrySpan, 0, spanCount)
	
	t := time.Now()
	fmt.Println(t, "Begin formatting spans ", spanCount)

	for i := 0; i < resourcespans.Len(); i++ {
		rspans := resourcespans.At(i)
		instSpans := rspans.InstrumentationLibrarySpans()
		// processing root id before sending in arh
		for j := 0; j < instSpans.Len(); j++ {
			ispans := instSpans.At(j)
			ispanItems := ispans.Spans()

			for k := 0; k < ispanItems.Len(); k++ {
				span := ispanItems.At(k)
				var rootSpanId string
				var traceId string
				if span.ParentSpanID().IsEmpty() {
					//rootSpanId = span.SpanID().HexString()
					traceId = span.TraceID().HexString()
					rootSpanId = span.Name()
					//parentSpanId = "nil"
					rootSpanList[traceId] = rootSpanId
					//fmt.Println("Formatting spans: Trace/Root:  ", traceId," / ", rootSpanId)
				} 
			}
		}
	}

	t = time.Now()
	fmt.Println(t, "Completed formatting meta-data")
	for i := 0; i < resourcespans.Len(); i++ {
		rspans := resourcespans.At(i)
		resource := rspans.Resource()
		resourceAttr := resource.Attributes().AsRaw()

		var serviceName, telSDKLang, telSDKName string
		if val, found := resourceAttr["service.name"]; found {
			serviceName = val.(string)
		}
		if val, found := resourceAttr["telemetry.sdk.name"]; found {
			telSDKName = val.(string)
		}
		if val, found := resourceAttr["telemetry.sdk.language"]; found {
			telSDKLang = val.(string)
		}

		instSpans := rspans.InstrumentationLibrarySpans()
		
		for j := 0; j < instSpans.Len(); j++ {
			ispans := instSpans.At(j)
			instLibName := ispans.InstrumentationLibrary().Name()
			instLibVer := ispans.InstrumentationLibrary().Version()
			ispanItems := ispans.Spans()

			for k := 0; k < ispanItems.Len(); k++ {
				span := ispanItems.At(k)
				
				//traceId := span.TraceID().HexString()
				//spanId := span.SpanID().HexString()
				//parentSpanId := span.ParentSpanID().HexString()

				rootSpanId := rootSpanList[span.TraceID().HexString()]

				//fmt.Println("Formatting spans: Trace/Span/Parent/Root:  ", traceId," / ", spanId," / ", parentSpanId," / ", rootSpanId)
				
				s247span := e.CreateTelemetrySpan(span, resourceAttr,
					serviceName,
					instLibName, instLibVer,
					telSDKLang, telSDKName, rootSpanId)
				spanList = append(spanList, s247span)
			}
		}
	}
	t = time.Now()
	fmt.Println(t, "Completed formatting spans ", spanCount)
	io.WriteString(e.file, "\nTransformed telemetry data to site24x7 format. \n")
	buf, err := json.Marshal(spanList)
	if err != nil {
		io.WriteString(e.file, "\nError in converting telemetry data. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}
	
	if strings.Contains(e.url, "catalyst") {
		err = e.SendCatalyst(buf)
		t = time.Now()
		fmt.Println(t, "Completed exporting spans catalyst", spanCount)
	} else {
		err = SendAppLogs(e, buf, len(spanList))
		t = time.Now()
		fmt.Println(t, "Completed exporting spans applogs", spanCount)
	}

	return err
}

func (e *site24x7exporter) SendCatalyst(buf []byte) error {
	// Deprecated end-point. 
	var urlBuf bytes.Buffer
	responseBody := bytes.NewBuffer(buf)
	//fmt.Println("Sending to Site24x7: ", responseBody)
	fmt.Fprint(&urlBuf, e.url, "?license.key=", e.apikey)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: e.insecure}
	resp, err := http.Post(urlBuf.String(), "application/json", responseBody)
	if err != nil {
		io.WriteString(e.file, "\nError in posting data to url. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}
	io.WriteString(e.file, "\nPosting telemetry data to url. \n")
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

func SendAppLogs(e *site24x7exporter, buf []byte, spanCount int) error {
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
		"X-LogType": []string{"s247apmopentelemetrytracing"},
		"X-StreamMode": []string{"1"},
		"Log-Size": []string{strconv.Itoa(spanCount)},
		"Content-Encoding": []string{"gzip"},
		"User-Agent": []string{"site24x7exporter"},
	}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: e.insecure}
	res , err := client.Do(req)
	if err != nil {
		//Handle Error
		fmt.Println("Error initializing Url: ", err)
		io.WriteString(e.file, "\nError in posting traces to url. \n")
		errstr := err.Error()
		io.WriteString(e.file, errstr)
		return err
	}
	io.WriteString(e.file, "\nPosting telemetry traces to url. \n")
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