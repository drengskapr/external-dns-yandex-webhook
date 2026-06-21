/*
Copyright 2025 YIVA BULUT.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"external-dns-yandex-webhook/internal/yandex/client"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

// yandex provider type
type yandexProvider struct {
	provider.BaseProvider
	client client.YandexDNSClient

	// only consider hosted zones managing domains ending in this suffix
	domainFilter *endpoint.DomainFilter
	dryRun       bool

	// TTL applied to records whose endpoint does not specify one
	defaultTTL int64
}

// NewYandexProvider initializes a new Yandex Cloud DNS based Provider
func NewYandexProvider(domainFilter *endpoint.DomainFilter, dryRun bool, defaultTTL int64, client client.YandexDNSClient) provider.Provider {
	return &yandexProvider{
		client:       client,
		domainFilter: domainFilter,
		dryRun:       dryRun,
		defaultTTL:   defaultTTL,
	}
}

// normalizeDNSName converts a DNS name to a canonical form, so that we can use string equality
// it: removes space, converts to lower case, ensures there is a trailing dot
func normalizeDNSName(dnsName string) string {
	s := strings.TrimSpace(strings.ToLower(dnsName))
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

// getZones returns a map of zone name to zone ID
func (p yandexProvider) getZones(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)

	zones, err := p.client.ListZones(ctx)
	if err != nil {
		return nil, err
	}

	for _, zone := range zones {
		if !p.domainFilter.Match(zone.Name) {
			continue
		}
		result[zone.Name] = zone.ID
	}

	return result, nil
}

func (p yandexProvider) getHostZoneID(hostname string, managedZones map[string]string) (string, error) {
	longestZoneLength := 0
	resultID := ""

	hostname = normalizeDNSName(hostname)

	for zoneName, zoneID := range managedZones {
		normalizedZoneName := normalizeDNSName(zoneName)
		if strings.HasSuffix(hostname, normalizedZoneName) {
			if len(zoneName) > longestZoneLength {
				longestZoneLength = len(zoneName)
				resultID = zoneID
			}
		}
	}

	if resultID == "" {
		return "", fmt.Errorf("no matching zone found for %s", hostname)
	}

	return resultID, nil
}

func (p yandexProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	zones, err := p.getZones(ctx)
	if err != nil {
		return nil, err
	}

	var endpoints []*endpoint.Endpoint

	for zoneName, zoneID := range zones {
		recordSets, err := p.client.ListRecordSets(ctx, zoneID)
		if err != nil {
			return nil, err
		}

		for _, recordSet := range recordSets {
			if recordSet.Type == "SOA" || recordSet.Type == "NS" {
				continue
			}

			name := recordSet.Name
			if !strings.HasSuffix(name, zoneName) {
				name = name + "." + zoneName
			}

			name = normalizeDNSName(name)

			ep := endpoint.NewEndpointWithTTL(name, recordSet.Type, endpoint.TTL(recordSet.TTL), recordSet.Data...)

			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, nil
}

type recordSet struct {
	dnsName     string
	recordType  string
	zoneID      string
	recordSetID string
	delete      bool
	ttl         int64
	names       map[string]bool
}

func addEndpoint(ep *endpoint.Endpoint, recordSets map[string]*recordSet, delete bool) {
	key := fmt.Sprintf("%s-%s", ep.DNSName, ep.RecordType)
	if _, ok := recordSets[key]; !ok {
		recordSets[key] = &recordSet{
			dnsName:    ep.DNSName,
			recordType: ep.RecordType,
			names:      make(map[string]bool),
		}
	}

	rs := recordSets[key]
	rs.delete = delete

	// Honor the endpoint's TTL when external-dns provides one; otherwise leave
	// it zero so upsertRecordSet falls back to the configured default.
	if ep.RecordTTL.IsConfigured() {
		rs.ttl = int64(ep.RecordTTL)
	}

	// Both replacements and deletions must carry the record data: Yandex
	// validates deletions by content and rejects an empty data set.
	for _, target := range ep.Targets {
		rs.names[target] = true
	}
}

func (p yandexProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	zones, err := p.getZones(ctx)
	if err != nil {
		return err
	}

	recordSets := make(map[string]*recordSet)

	for _, ep := range changes.Create {
		addEndpoint(ep, recordSets, false)
	}

	for _, ep := range changes.UpdateNew {
		addEndpoint(ep, recordSets, false)
	}

	for _, ep := range changes.Delete {
		addEndpoint(ep, recordSets, true)
	}

	for _, rs := range recordSets {
		err := p.upsertRecordSet(ctx, rs, zones)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p yandexProvider) upsertRecordSet(ctx context.Context, rs *recordSet, managedZones map[string]string) error {
	if rs.zoneID == "" {
		var err error
		rs.zoneID, err = p.getHostZoneID(rs.dnsName, managedZones)
		if err != nil {
			return err
		}
	}

	var records []string
	for name := range rs.names {
		records = append(records, name)
	}

	ttl := rs.ttl
	if ttl == 0 {
		ttl = p.defaultTTL
	}

	recordSet := client.RecordSet{
		Name: normalizeDNSName(rs.dnsName),
		Type: rs.recordType,
		TTL:  ttl,
		Data: records,
	}

	if rs.delete {
		if p.dryRun {
			log.Infof("Would delete record set %s with type %s in zone %s",
				rs.dnsName, rs.recordType, rs.zoneID)
			return nil
		}

		req := client.UpsertRequest{
			DnsZoneID: rs.zoneID,
			Deletions: []client.RecordSet{recordSet},
		}

		return p.client.UpsertRecordSets(ctx, req)
	}

	if p.dryRun {
		operation := "update"
		if rs.recordSetID == "" {
			operation = "create"
		}
		log.Infof("Would %s record set %s with type %s in zone %s",
			operation,
			rs.dnsName,
			rs.recordType,
			rs.zoneID)
		return nil
	}

	req := client.UpsertRequest{
		DnsZoneID:    rs.zoneID,
		Replacements: []client.RecordSet{recordSet},
	}

	return p.client.UpsertRecordSets(ctx, req)
}
