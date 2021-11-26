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


func (e *site24x7exporter) CreateTelemetrySpan(span pdata.Span, resourceAttr map[string]interface{}, serviceName string) (TelemetrySpan) {
	
	spanAttr := span.Attributes().AsRaw()
	startTime := (span.StartTimestamp().AsTime().UnixNano()) // int64(time.Millisecond))
	endTime := (span.EndTimestamp().AsTime().UnixNano()) // int64(time.Millisecond))
	spanEvts := span.Events()
	telEvents := make([]TelemetrySpanEvent,0,spanEvts.Len())
	for i := 0; i < spanEvts.Len(); i++ {
		spanEvt := spanEvts.At(i)
		telEvt := TelemetrySpanEvent{
			Timestamp: (spanEvt.Timestamp().AsTime().UnixNano() / int64(time.Millisecond)),
			Name: spanEvt.Name(),
			EventAttributes: spanEvt.Attributes().AsRaw(),
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
		instSpans := rspans.InstrumentationLibrarySpans()
		for j := 0; j < instSpans.Len(); j++ {
			ispans := instSpans.At(j)
			ispanItems := ispans.Spans()
			for k := 0; k < ispanItems.Len(); k++ {
				span := ispanItems.At(k)
				s247span := e.CreateTelemetrySpan(span, resourceAttr, serviceName)
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
