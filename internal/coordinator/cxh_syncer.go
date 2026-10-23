package coordinator

import (
	"context"
	"net"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/cloudflare"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/logger"
	"go.uber.org/zap"
)

// cdnRecordsPerNode is how many subdomains a node keeps: the newest one goes
// into fresh subscriptions, the previous one keeps older clients working for
// one more rotation interval.
const cdnRecordsPerNode = 2

// cdnSyncer keeps the Cloudflare side of Remote CXH in step with the
// settings and nodes: one Origin CA certificate for the CDN domain, and
// proxied subdomains of it pointing at every node, rotated on an interval.
// Nothing here touches Xray; the composer reads the stored state.
type cdnSyncer struct {
	l             *logger.Logger
	db            *data.Store
	baseUrl       string
	onCertificate func()
}

// nodeCdnInfo is a snapshot of the CDN-relevant fields of a node.
type nodeCdnInfo struct {
	id   string
	host string
}

// newCdnSyncer creates a new CDN syncer. baseUrl overrides the Cloudflare API
// URL (tests); onCertificate runs after a certificate or path change, which
// is the only CDN change the node configs depend on.
func newCdnSyncer(l *logger.Logger, db *data.Store, baseUrl string, onCertificate func()) *cdnSyncer {
	return &cdnSyncer{l: l, db: db, baseUrl: baseUrl, onCertificate: onCertificate}
}

// syncCdn brings Cloudflare and the stored CDN state in line with the
// settings. API calls run without the store lock; the state is written back
// at the end, so a crash mid-way at worst leaves a record to clean up later.
func (s *cdnSyncer) syncCdn() error {
	var port, rotationHours int
	var token, domain string
	var cdn data.Cdn
	var nodes []nodeCdnInfo
	s.db.Read(func(d *data.Data) {
		port = d.XraySettings.RemoteCxhPort
		token = d.MainSettings.CloudflareToken
		domain = d.MainSettings.CdnDomain
		rotationHours = d.MainSettings.CdnRotationHours
		cdn = *d.Cdn
		cdn.Records = append([]*data.CdnRecord{}, d.Cdn.Records...)
		for _, n := range d.Nodes {
			nodes = append(nodes, nodeCdnInfo{id: n.Id, host: n.Host})
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var client *cloudflare.Client
	if token != "" {
		client = cloudflare.New(token)
		if s.baseUrl != "" {
			client = client.WithBaseUrl(s.baseUrl)
		}
	}

	enabled := port > 0 && token != "" && domain != ""
	certificateChanged := false
	var errList error

	// Method off, or the domain changed: drop the records (and on a domain
	// change the certificate) made for the old domain. Only what Cloudflare
	// confirmed gone is forgotten; the rest stays stored and is retried on the
	// next run, and a domain change waits until the old domain is clean. With
	// no token nothing can be deleted, so the state is kept for when one is set.
	if !enabled || cdn.Domain != domain {
		if client == nil || (cdn.ZoneId == "" && len(cdn.Records) > 0) {
			return errors.WithStack(s.store(cdn, false))
		}
		kept := make([]*data.CdnRecord, 0, len(cdn.Records))
		for _, r := range cdn.Records {
			if err := s.deleteRecord(ctx, client, cdn.ZoneId, r); err != nil {
				errList = errors.Join(errList, err)
				kept = append(kept, r)
			}
		}
		cdn.Records = kept
		if cdn.Domain != domain && cdn.CertificateId != "" {
			if err := client.RevokeOriginCertificate(ctx, cdn.CertificateId); err != nil && !cloudflare.IsNotFound(err) {
				errList = errors.Join(errList, err)
			} else {
				cdn.CertificateId = ""
			}
		}
		if len(cdn.Records) > 0 || (cdn.Domain != domain && cdn.CertificateId != "") {
			return errors.Join(errList, s.store(cdn, false))
		}
		if cdn.Domain != domain {
			cdn = data.Cdn{CxhPath: cdn.CxhPath, Domain: domain, Records: []*data.CdnRecord{}}
			certificateChanged = true
		}
	}

	if enabled {
		if cdn.CxhPath == "" {
			cdn.CxhPath = "/" + util.ShortId() + util.ShortId()
			certificateChanged = true
		}
		if cdn.ZoneId == "" {
			zone, err := client.FindZone(ctx, domain)
			if err != nil {
				return errors.Join(errList, s.store(cdn, certificateChanged), err)
			}
			cdn.ZoneId = zone.Id
		}
		if !cdn.Ready() {
			privateKey, csr, err := cloudflare.GenerateCsr(cloudflare.OriginHostnames(domain))
			if err != nil {
				return errors.Join(errList, s.store(cdn, certificateChanged), err)
			}
			certificate, err := client.CreateOriginCertificate(ctx, csr, cloudflare.OriginHostnames(domain), cloudflare.OriginValidityDays)
			if err != nil {
				return errors.Join(errList, s.store(cdn, certificateChanged), err)
			}
			cdn.CertificateId = certificate.Id
			cdn.Certificate = certificate.Certificate
			cdn.PrivateKey = privateKey
			certificateChanged = true
			s.l.Info("coordinator: cdn certificate issued", zap.String("domain", domain), zap.String("expires", certificate.ExpiresOn))
		}

		errList = errors.Join(errList, s.syncRecords(ctx, client, &cdn, nodes, rotationHours))
	}

	return errors.Join(errList, s.store(cdn, certificateChanged))
}

// syncRecords gives every node a current subdomain and removes the rest:
// records of gone nodes, records pointing at an old host, and all but the
// newest cdnRecordsPerNode after a rotation.
func (s *cdnSyncer) syncRecords(
	ctx context.Context,
	client *cloudflare.Client,
	cdn *data.Cdn,
	nodes []nodeCdnInfo,
	rotationHours int,
) error {
	var errList error
	now := time.Now()
	hosts := map[string]string{}
	for _, n := range nodes {
		hosts[n.id] = n.host
	}

	kept := make([]*data.CdnRecord, 0, len(cdn.Records))
	for _, r := range cdn.Records {
		if host, ok := hosts[r.NodeId]; !ok || host != r.Content {
			if err := s.deleteRecord(ctx, client, cdn.ZoneId, r); err != nil {
				errList = errors.Join(errList, err)
				kept = append(kept, r) // retry next run
			}
			continue
		}
		kept = append(kept, r)
	}
	cdn.Records = kept

	for _, n := range nodes {
		records := cdn.NodeRecords(n.id)
		rotate := rotationHours > 0 && len(records) > 0 &&
			now.Sub(time.UnixMilli(records[len(records)-1].CreatedAt)) >= time.Duration(rotationHours)*time.Hour
		if len(records) == 0 || rotate {
			record, err := s.createRecord(ctx, client, cdn, n, now)
			if err != nil {
				errList = errors.Join(errList, err)
				continue
			}
			cdn.Records = append(cdn.Records, record)
			records = append(records, record)
		}
		for len(records) > cdnRecordsPerNode {
			oldest := records[0]
			if err := s.deleteRecord(ctx, client, cdn.ZoneId, oldest); err != nil {
				errList = errors.Join(errList, err)
				break
			}
			records = records[1:]
			cdn.Records = removeRecord(cdn.Records, oldest)
		}
	}

	return errList
}

// createRecord adds a proxied record on a random subdomain for the node: an A
// record for an IP host, a CNAME for a hostname.
func (s *cdnSyncer) createRecord(
	ctx context.Context,
	client *cloudflare.Client,
	cdn *data.Cdn,
	n nodeCdnInfo,
	now time.Time,
) (*data.CdnRecord, error) {
	recordType := "CNAME"
	if ip := net.ParseIP(n.host); ip != nil {
		recordType = "A"
		if ip.To4() == nil {
			recordType = "AAAA"
		}
	}
	name := util.ShortId() + "." + cdn.Domain
	created, err := client.CreateDnsRecord(ctx, cdn.ZoneId, cloudflare.DnsRecord{
		Type:    recordType,
		Name:    name,
		Content: n.host,
		Proxied: true,
		Ttl:     1,
		Comment: "p-manager node " + n.id,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s.l.Info("coordinator: cdn record created", zap.String("name", name), zap.String("node", n.id))
	return &data.CdnRecord{
		Id:        created.Id,
		NodeId:    n.id,
		Name:      name,
		Content:   n.host,
		CreatedAt: now.UnixMilli(),
	}, nil
}

// deleteRecord removes a record; one already gone counts as deleted.
func (s *cdnSyncer) deleteRecord(ctx context.Context, client *cloudflare.Client, zoneId string, r *data.CdnRecord) error {
	err := client.DeleteDnsRecord(ctx, zoneId, r.Id)
	if err != nil && !cloudflare.IsNotFound(err) {
		return errors.WithStack(err)
	}
	s.l.Info("coordinator: cdn record deleted", zap.String("name", r.Name), zap.String("node", r.NodeId))
	return nil
}

// store writes the CDN state back and, when the certificate or path changed,
// lets the node configs follow.
func (s *cdnSyncer) store(cdn data.Cdn, certificateChanged bool) error {
	var changed bool
	err := s.db.Mutate(func(d *data.Data) (bool, error) {
		changed = !cdnEqual(d.Cdn, &cdn)
		if changed {
			d.Cdn = &cdn
		}
		return changed, nil
	})
	if err != nil {
		return errors.WithStack(err)
	}
	if changed && certificateChanged && s.onCertificate != nil {
		s.onCertificate()
	}
	return nil
}

func cdnEqual(a, b *data.Cdn) bool {
	if a.CxhPath != b.CxhPath || a.Domain != b.Domain || a.ZoneId != b.ZoneId ||
		a.CertificateId != b.CertificateId || a.Certificate != b.Certificate || a.PrivateKey != b.PrivateKey ||
		len(a.Records) != len(b.Records) {
		return false
	}
	for i := range a.Records {
		if *a.Records[i] != *b.Records[i] {
			return false
		}
	}
	return true
}

func removeRecord(records []*data.CdnRecord, r *data.CdnRecord) []*data.CdnRecord {
	out := records[:0]
	for _, x := range records {
		if x != r {
			out = append(out, x)
		}
	}
	return out
}
