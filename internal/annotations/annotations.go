// Package annotations reads the google.api resource extensions off
// descriptors without panicking when they are absent.
package annotations

import (
	apipb "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Resource returns the (google.api.resource) option of a message, or nil.
func Resource(m protoreflect.MessageDescriptor) *apipb.ResourceDescriptor {
	opts, ok := m.Options().(*descriptorpb.MessageOptions)
	if !ok || opts == nil || !proto.HasExtension(opts, apipb.E_Resource) {
		return nil
	}
	rd, _ := proto.GetExtension(opts, apipb.E_Resource).(*apipb.ResourceDescriptor)
	return rd
}

// ResourceDefinitions returns the (google.api.resource_definition) options
// of a file in declaration order, or nil.
func ResourceDefinitions(f protoreflect.FileDescriptor) []*apipb.ResourceDescriptor {
	opts, ok := f.Options().(*descriptorpb.FileOptions)
	if !ok || opts == nil || !proto.HasExtension(opts, apipb.E_ResourceDefinition) {
		return nil
	}
	defs, _ := proto.GetExtension(opts, apipb.E_ResourceDefinition).([]*apipb.ResourceDescriptor)
	return defs
}

// ResourceReference returns the (google.api.resource_reference) option of a
// field, or nil.
func ResourceReference(fd protoreflect.FieldDescriptor) *apipb.ResourceReference {
	opts, ok := fd.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil || !proto.HasExtension(opts, apipb.E_ResourceReference) {
		return nil
	}
	ref, _ := proto.GetExtension(opts, apipb.E_ResourceReference).(*apipb.ResourceReference)
	return ref
}
