package tools

import "strings"

// CLM (Constrained Language Mode) allowed types for PowerShell .NET type casts.
// Types NOT in this set trigger an ask — they access system APIs CLM blocks.
//
// SECURITY: 'adsi', 'adsisearcher', 'wmi', 'wmiclass', 'wmisearcher', 'cimsession'
// are REMOVED. These types perform NETWORK BINDS when cast:
//   [adsi]'LDAP://evil.com/...'  -> connects to LDAP server
//   [wmi]'\\evil-host\root\cimv2:Win32_Process.Handle="1"' -> remote WMI
//
// Source: clmTypes.ts:18-188 — CLM_ALLOWED_TYPES
var CLMAllowedTypes = func() map[string]bool {
	types := []string{
		// Type accelerators (short names)
		"alias", "allowemptycollection", "allowemptystring", "allownull",
		"argumentcompleter", "argumentcompletions", "array", "bigint",
		"bool", "byte", "char", "cimclass", "cimconverter", "ciminstance",
		// 'cimsession' REMOVED — network bind hazard
		"cimtype", "cmdletbinding", "cultureinfo", "datetime", "decimal",
		"double", "dsclocalconfigurationmanager", "dscproperty", "dscresource",
		"experimentaction", "experimental", "experimentalfeature",
		"float", "guid", "hashtable", "int", "int16", "int32", "int64",
		"ipaddress", "ipendpoint", "long", "mailaddress",
		"norunspaceaffinity", "nullstring", "objectsecurity", "ordered",
		"outputtype", "parameter", "physicaladdress",
		"pscredential", "pscustomobject", "psdefaultvalue", "pslistmodifier",
		"psobject", "psprimitivedictionary", "pstypenameattribute",
		"ref", "regex", "sbyte", "securestring", "semver", "short", "single",
		"string", "supportswildcards", "switch", "timespan",
		"uint", "uint16", "uint32", "uint64", "ulong", "uri", "ushort",
		"validatecount", "validatedrive", "validatelength",
		"validatenotnull", "validatenotnullorempty", "validatenotnullorwhitespace",
		"validatepattern", "validaterange", "validatescript", "validateset",
		"validatetrusteddata", "validateuserdrive",
		"version", "void", "wildcardpattern",
		// 'wmi', 'wmiclass', 'wmisearcher' REMOVED — network bind hazard
		"x500distinguishedname", "x509certificate", "xml",

		// Full names for accelerators that resolve to System.*
		"system.array", "system.boolean", "system.byte", "system.char",
		"system.datetime", "system.decimal", "system.double", "system.guid",
		"system.int16", "system.int32", "system.int64",
		"system.numerics.biginteger",
		"system.sbyte", "system.single", "system.string", "system.timespan",
		"system.uint16", "system.uint32", "system.uint64",
		"system.uri", "system.version", "system.void",
		"system.collections.hashtable",
		"system.text.regularexpressions.regex",
		"system.globalization.cultureinfo",
		"system.net.ipaddress", "system.net.ipendpoint",
		"system.net.mail.mailaddress",
		"system.net.networkinformation.physicaladdress",
		"system.security.securestring",
		"system.security.cryptography.x509certificates.x509certificate",
		"system.security.cryptography.x509certificates.x500distinguishedname",
		"system.xml.xmldocument",

		// System.Management.Automation.*
		"system.management.automation.pscredential",
		"system.management.automation.pscustomobject",
		"system.management.automation.pslistmodifier",
		"system.management.automation.psobject",
		"system.management.automation.psprimitivedictionary",
		"system.management.automation.psreference",
		"system.management.automation.semanticversion",
		"system.management.automation.switchparameter",
		"system.management.automation.wildcardpattern",
		"system.management.automation.language.nullstring",

		// Microsoft.Management.Infrastructure.*
		// 'cimsession' FQ REMOVED — network bind hazard
		"microsoft.management.infrastructure.cimclass",
		"microsoft.management.infrastructure.cimconverter",
		"microsoft.management.infrastructure.ciminstance",
		"microsoft.management.infrastructure.cimtype",

		// FQ equivalents of remaining short-name accelerators
		// DirectoryEntry/DirectorySearcher/ManagementObject/* FQ REMOVED
		"system.collections.specialized.ordereddictionary",
		"system.security.accesscontrol.objectsecurity",

		// object types
		"object", "system.object",

		// ModuleSpecification
		"microsoft.powershell.commands.modulespecification",
	}

	m := make(map[string]bool, len(types))
	for _, t := range types {
		m[t] = true
	}
	return m
}()

// CLMRemovedTypes are types explicitly removed from the allowlist due to
// network-bind security hazard.
// Source: clmTypes.ts — SECURITY comments
var CLMRemovedTypes = map[string]bool{
	"adsi": true, "adsisearcher": true,
	"wmi": true, "wmiclass": true, "wmisearcher": true,
	"cimsession": true,
	// FQ equivalents
	"system.directoryservices.directoryentry":                     true,
	"system.directoryservices.directorysearcher":                  true,
	"system.management.managementobject":                          true,
	"system.management.managementclass":                           true,
	"system.management.managementobjectsearcher":                  true,
	"microsoft.management.infrastructure.cimsession":              true,
}

// NormalizePSTypeName normalizes a PS type name from AST.
// Handles array suffix ([]) and generic brackets.
// Source: clmTypes.ts:194-203 — normalizeTypeName()
func NormalizePSTypeName(name string) string {
	lower := strings.ToLower(name)
	// Strip array suffix: "String[]" -> "string"
	lower = strings.TrimSuffix(lower, "[]")
	// Strip generic args: "List[int]" -> "list"
	if idx := strings.Index(lower, "["); idx >= 0 {
		lower = lower[:idx]
	}
	return strings.TrimSpace(lower)
}

// IsClmAllowedType returns true if the type name is in the CLM allowlist.
// Types NOT in this set trigger ask — they access system APIs CLM blocks.
// Source: clmTypes.ts:209-211 — isClmAllowedType()
func IsClmAllowedType(typeName string) bool {
	return CLMAllowedTypes[NormalizePSTypeName(typeName)]
}
