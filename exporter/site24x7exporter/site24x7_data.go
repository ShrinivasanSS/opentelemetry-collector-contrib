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

type telemetryAttributes = map[string]interface{}
type TelemetrySpanEvent struct{
	Timestamp int64 `json:"timestamp"`
	Name string `json:"name"`
	EventAttributes telemetryAttributes `json:"eventAttributes"`
}
type TelemetrySpanLink struct{
	LinkSpanID string `json:"link.spanID"`
	LinkTraceID string `json:"link.traceID"`
}
type TelemetrySpan struct{
	TraceId string `json:"trace_id"`
	SpanId string `json:"span_id"`
	ParentSpanId string `json:"parent_id"`
	Name string `json:"name"`
	Kind string `json:"span_kind"`
	StartTime int64 `json:"start_time"`
	EndTime int64 `json:"end_time"`
	Duration int64 `json:"duration"`

	// resource->attributes[]->key('service.name')
	ServiceName string `json:"service_name"`
	
	// Events[]->eventAttributes->exception.message
	ExceptionMessage []string `json:"exception_message"`
	// Events[]->eventAttributes->exception.stacktrace
	ExceptionStackTrace []string `json:"stack_trace"`
	// Events[]->eventAttributes->exception.type
	ExceptionType []string `json:"exception_class"`

	// instrumentationLibrarySpans[]->instrumentationLibrary->name
	InstrumentationLibrary string `json:"instrumentation_name"`
	// instrumentationLibrarySpans[]->instrumentationLibrary->name
	InstrumentationLibraryVersion string `json:"instrumentation_version"`
	// resource->attributes[]->key('telemetry.sdk.language')
	TelemetrySDKLanguage string `json:"service_type"`
	// resource->attributes[]->key('telemetry.sdk.name'). Should be opentelemetry. 
	TelemetrySDKName string `json:"log_sub_type"`
	// resource->attributes[]->key('telemetry.sdk.version')
	//TelemetrySDKVersion string  `json:"instrumentation_version"`
	
	// spans->attributes[]->key('net.peer.ip')
	HostIP string `json:"host_ip"`
	// spans->attributes[]->key('net.peer.name')
	HostName string `json:"host_name"`
	// spans->attributes[]->key('net.peer.port')
	HostPort string `json:"host_port"`
	// spans->attributes[]->key('thread.id')
	ThreadId string `json:"thread_id"`
	// spans->attributes[]->key('thread.name')
	ThreadName string  `json:"thread_name"`
	// spans->attributes[]->key('db.system')
	DbSystem string `json:"type"`
	// spans->attributes[]->key('db.statement')
	DbStatement string `json:"db_statement"`
	// spans->attributes[]->key('db.name')
	DbName string `json:"db_name"`
	// spans->attributes[]->key('db.connection_string')
	DbConnStr string `json:"connection_string"`
	// spans->attributes[]->key('http.url') or name. 
	HttpUrl string `json:"url"`
	// spans->attributes[]->key('http.method') 
	HttpMethod string `json:"http_method"`
	// spans->attributes[]->key('http.status_code') 
	HttpStatusCode string `json:"http_status_code"`

	// if parentspanid is empty. 
	IsRoot bool `json:"root"`

	ResourceAttributes telemetryAttributes  `json:"ResourceAttributes"`
	SpanAttributes telemetryAttributes  `json:"SpanAttributes"`
	TraceState string `json:"TraceState"`
	Events []TelemetrySpanEvent `json:"Events"`
	Links []TelemetrySpanLink `json:"Links"`
	StatusCode string `json:"status.code"`
	StatusMsg string `json:"status.msg"`
	DroppedAttributesCount uint32 `json:"DroppedAttributesCount"`
	DroppedLinksCount uint32 `json:"DroppedLinksCount"`
	DroppedEventsCount uint32 `json:"DroppedEventsCount"`
}

type TelemetryLog struct {
	TraceId string `json:"TraceId"`
	SpanId string `json:"SpanId"`
	Timestamp int64 `json:"_zl_timestamp"`
	S247UID string `json:"s247agentuid"`
	Name string `json:"name"`
	LogLevel string `json:"LogLevel"`
	Message string `json:"Message"`
	LogAttributes telemetryAttributes `json:"attributes"`
	ResourceAttributes telemetryAttributes  `json:"ResourceAttributes"`
	DroppedAttributesCount uint32 `json:"DroppedAttributesCount"`
	TraceFlag uint32 `json:"TraceFlag"`
}