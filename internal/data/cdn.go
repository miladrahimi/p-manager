package data

// Cdn is the state of Remote CXH that P-Manager keeps itself (no UI edits):
// the XHTTP path, the Cloudflare zone and Origin CA certificate, and the proxied
// subdomains pointing at the nodes.
type Cdn struct {
	CxhPath string `json:"cxh_path"`
	// Domain is the CdnDomain the zone and certificate below were made for.
	Domain        string       `json:"domain"`
	ZoneId        string       `json:"zone_id"`
	CertificateId string       `json:"certificate_id"`
	Certificate   string       `json:"certificate"`
	PrivateKey    string       `json:"private_key"`
	Records       []*CdnRecord `json:"records"`
}

// CdnRecord is a proxied DNS record (one subdomain) pointing at a node.
type CdnRecord struct {
	Id        string `json:"id"`
	NodeId    string `json:"node_id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
}

// DefaultCdn returns an empty CDN state.
func DefaultCdn() *Cdn {
	return &Cdn{Records: []*CdnRecord{}}
}

// Ready reports whether a certificate is in place for nodes to serve TLS with.
func (c *Cdn) Ready() bool {
	return c.CxhPath != "" && c.Certificate != "" && c.PrivateKey != ""
}

// NodeRecords returns the records of a node, oldest first.
func (c *Cdn) NodeRecords(nodeId string) []*CdnRecord {
	var records []*CdnRecord
	for _, r := range c.Records {
		if r.NodeId == nodeId {
			records = append(records, r)
		}
	}
	return records
}
