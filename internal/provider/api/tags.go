package api

import (
	"context"
	"fmt"
	"net/http"

	managementv1 "github.com/lakekeeper/go-lakekeeper/pkg/apis/management/v1"
)

// ---------------------------------------------------------------------------
// Tag Definitions

// TagDefinition is a Lakekeeper governance tag definition.
type TagDefinition struct {
	ID            string
	Name          string
	Description   *string
	Scope         []string
	ValueKind     string
	AllowedValues []string
	CreatedAt     string
	UpdatedAt     *string
}

// CreateTagDefinitionRequest carries the fields for CreateTagDefinition.
type CreateTagDefinitionRequest struct {
	Name          string
	Description   *string
	Scope         []string
	ValueKind     string
	AllowedValues []string
}

// UpdateTagDefinitionRequest carries the fields for UpdateTagDefinition.
type UpdateTagDefinitionRequest struct {
	Name             string
	Description      *string
	Scope            []string
	AddAllowedValues []string
}

func tagScopesToSDK(scopes []string) []managementv1.TagScope {
	out := make([]managementv1.TagScope, len(scopes))
	for i, s := range scopes {
		out[i] = managementv1.TagScope(s)
	}
	return out
}

func tagDefinitionFromSDK(d *managementv1.TagDefinition) *TagDefinition {
	scope := make([]string, len(d.Scope))
	for i, s := range d.Scope {
		scope[i] = string(s)
	}
	return &TagDefinition{
		ID:            d.Id,
		Name:          d.Name,
		Description:   d.Description.Get(),
		Scope:         scope,
		ValueKind:     string(d.ValueKind),
		AllowedValues: d.AllowedValues,
		CreatedAt:     formatTime(d.CreatedAt),
		UpdatedAt:     formatNullableTime(d.UpdatedAt),
	}
}

// CreateTagDefinition creates a governance tag definition in projectID.
func (c *Client) CreateTagDefinition(ctx context.Context, projectID string, req CreateTagDefinitionRequest) (*TagDefinition, error) {
	body := managementv1.NewCreateTagDefinitionRequest(req.Name, tagScopesToSDK(req.Scope), managementv1.TagValueKind(req.ValueKind))
	if req.Description != nil {
		body.SetDescription(*req.Description)
	}
	if len(req.AllowedValues) > 0 {
		body.SetAllowedValues(req.AllowedValues)
	}

	r := c.sdk.TagAPI.CreateTagDefinition(ctx).CreateTagDefinitionRequest(*body)
	if projectID != "" {
		r = r.XProjectId(projectID)
	}
	d, resp, err := r.Execute()
	if err != nil {
		return nil, wrapGenErr("create tag definition", resp, err)
	}
	return tagDefinitionFromSDK(d), nil
}

// GetTagDefinition returns a governance tag definition by ID.
func (c *Client) GetTagDefinition(ctx context.Context, projectID, tagDefinitionID string) (*TagDefinition, error) {
	r := c.sdk.TagAPI.GetTagDefinition(ctx, tagDefinitionID)
	if projectID != "" {
		r = r.XProjectId(projectID)
	}
	d, resp, err := r.Execute()
	if err != nil {
		return nil, wrapGenErr("get tag definition", resp, err)
	}
	return tagDefinitionFromSDK(d), nil
}

// TagDefinitionExists reports whether a tag definition with the given ID exists.
func (c *Client) TagDefinitionExists(ctx context.Context, tagDefinitionID string) (bool, error) {
	r := c.sdk.TagAPI.GetTagDefinition(ctx, tagDefinitionID)
	_, resp, err := r.Execute()
	if err != nil {
		wrapped := wrapGenErr("get tag definition", resp, err)
		if IsNotFound(wrapped) {
			return false, nil
		}
		return false, wrapped
	}
	return true, nil
}

// ListTagDefinitions lists all tag definitions in projectID, optionally filtered by exact name.
func (c *Client) ListTagDefinitions(ctx context.Context, projectID string, name *string) ([]TagDefinition, error) {
	var out []TagDefinition
	var pageToken *string
	for {
		r := c.sdk.TagAPI.ListTagDefinitions(ctx)
		if projectID != "" {
			r = r.XProjectId(projectID)
		}
		if name != nil {
			r = r.Name(*name)
		}
		if pageToken != nil {
			r = r.PageToken(*pageToken)
		}
		res, resp, err := r.Execute()
		if err != nil {
			return nil, wrapGenErr("list tag definitions", resp, err)
		}
		for i := range res.TagDefinitions {
			out = append(out, *tagDefinitionFromSDK(&res.TagDefinitions[i]))
		}
		if next, ok := res.GetNextPageTokenOk(); ok && next != nil && *next != "" {
			pageToken = next
			continue
		}
		break
	}
	return out, nil
}

// UpdateTagDefinition updates a governance tag definition.
func (c *Client) UpdateTagDefinition(ctx context.Context, projectID, tagDefinitionID string, req UpdateTagDefinitionRequest) (*TagDefinition, error) {
	body := managementv1.NewUpdateTagDefinitionRequest(req.Name, tagScopesToSDK(req.Scope))
	if req.Description != nil {
		body.SetDescription(*req.Description)
	}
	if len(req.AddAllowedValues) > 0 {
		body.SetAddAllowedValues(req.AddAllowedValues)
	}

	r := c.sdk.TagAPI.UpdateTagDefinition(ctx, tagDefinitionID).UpdateTagDefinitionRequest(*body)
	if projectID != "" {
		r = r.XProjectId(projectID)
	}
	d, resp, err := r.Execute()
	if err != nil {
		return nil, wrapGenErr("update tag definition", resp, err)
	}
	return tagDefinitionFromSDK(d), nil
}

// DeleteTagDefinition deletes a governance tag definition.
func (c *Client) DeleteTagDefinition(ctx context.Context, projectID, tagDefinitionID string) error {
	r := c.sdk.TagAPI.DeleteTagDefinition(ctx, tagDefinitionID)
	if projectID != "" {
		r = r.XProjectId(projectID)
	}
	resp, err := r.Execute()
	if err != nil {
		return wrapGenErr("delete tag definition", resp, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tag Attachments & Effective Tags

// AppliedTag is returned when attaching or updating a tag on a target.
type AppliedTag struct {
	TagDefinitionID string
	Name            string
	Value           *string
	AppliedAt       string
}

func appliedTagFromSDK(t *managementv1.AppliedTag) *AppliedTag {
	return &AppliedTag{
		TagDefinitionID: t.TagDefinitionId,
		Name:            t.Name,
		Value:           t.Value.Get(),
		AppliedAt:       formatTime(t.AppliedAt),
	}
}

// TagInheritanceSource identifies the ancestor a tag was inherited from.
type TagInheritanceSource struct {
	Type        string
	WarehouseID string
	NamespaceID *string
}

// TargetTag is a tag attached to or inherited by a target object.
type TargetTag struct {
	TagDefinitionID string
	Name            string
	Value           *string
	Source          string
	CreatedAt       string
	UpdatedAt       *string
	InheritedFrom   *TagInheritanceSource
}

func targetTagFromSDK(t managementv1.TargetTag) TargetTag {
	var inh *TagInheritanceSource
	if src := t.InheritedFrom.Get(); src != nil {
		if src.TagInheritanceSourceWarehouse != nil {
			inh = &TagInheritanceSource{
				Type:        TagScopeWarehouse,
				WarehouseID: src.TagInheritanceSourceWarehouse.WarehouseId,
			}
		} else if src.TagInheritanceSourceNamespace != nil {
			inh = &TagInheritanceSource{
				Type:        TagScopeNamespace,
				WarehouseID: src.TagInheritanceSourceNamespace.WarehouseId,
				NamespaceID: new(src.TagInheritanceSourceNamespace.NamespaceId),
			}
		}
	}
	return TargetTag{
		TagDefinitionID: t.TagDefinitionId,
		Name:            t.Name,
		Value:           t.Value.Get(),
		Source:          string(t.Source),
		CreatedAt:       formatTime(t.CreatedAt),
		UpdatedAt:       formatNullableTime(t.UpdatedAt),
		InheritedFrom:   inh,
	}
}

func targetTagsFromSDK(res *managementv1.ListTagsResponse) []TargetTag {
	out := make([]TargetTag, len(res.Tags))
	for i, t := range res.Tags {
		out[i] = targetTagFromSDK(t)
	}
	return out
}

// ColumnTags groups direct tags on a single column by Iceberg field ID.
type ColumnTags struct {
	FieldID int32
	Tags    []TargetTag
}

// TagAttachmentTarget identifies the object a tag is attached to in a reverse lookup.
type TagAttachmentTarget struct {
	Type           string
	WarehouseID    string
	NamespaceID    string
	TableID        string
	ViewID         string
	GenericTableID string
	FieldID        *int32
}

// TagAttachment is one entry in a reverse-lookup listing for a tag definition.
type TagAttachment struct {
	Target    TagAttachmentTarget
	Value     *string
	Source    string
	CreatedAt string
	UpdatedAt *string
}

// ListTagAttachmentsFilter filters ListTagAttachments.
type ListTagAttachmentsFilter struct {
	TargetType  *string
	WarehouseID *string
	Value       *string
}

func tagAttachmentTargetFromSDK(t managementv1.TagAttachmentTarget) (TagAttachmentTarget, error) {
	switch {
	case t.TagAttachmentTargetWarehouse != nil:
		return TagAttachmentTarget{
			Type:        TagScopeWarehouse,
			WarehouseID: t.TagAttachmentTargetWarehouse.WarehouseId,
		}, nil
	case t.TagAttachmentTargetNamespace != nil:
		return TagAttachmentTarget{
			Type:        TagScopeNamespace,
			WarehouseID: t.TagAttachmentTargetNamespace.WarehouseId,
			NamespaceID: t.TagAttachmentTargetNamespace.NamespaceId,
		}, nil
	case t.TagAttachmentTargetTable != nil:
		return TagAttachmentTarget{
			Type:        TagScopeTable,
			WarehouseID: t.TagAttachmentTargetTable.WarehouseId,
			TableID:     t.TagAttachmentTargetTable.TableId,
		}, nil
	case t.TagAttachmentTargetView != nil:
		return TagAttachmentTarget{
			Type:        TagScopeView,
			WarehouseID: t.TagAttachmentTargetView.WarehouseId,
			ViewID:      t.TagAttachmentTargetView.ViewId,
		}, nil
	case t.TagAttachmentTargetGenericTable != nil:
		return TagAttachmentTarget{
			Type:           TagScopeGenericTable,
			WarehouseID:    t.TagAttachmentTargetGenericTable.WarehouseId,
			GenericTableID: t.TagAttachmentTargetGenericTable.GenericTableId,
		}, nil
	case t.TagAttachmentTargetColumn != nil:
		return TagAttachmentTarget{
			Type:        TagScopeColumn,
			WarehouseID: t.TagAttachmentTargetColumn.WarehouseId,
			TableID:     t.TagAttachmentTargetColumn.TableId,
			FieldID:     new(t.TagAttachmentTargetColumn.FieldId),
		}, nil
	default:
		return TagAttachmentTarget{}, fmt.Errorf("unknown tag attachment target")
	}
}

// ListTagAttachments returns all direct attachments of tagDefinitionID.
func (c *Client) ListTagAttachments(ctx context.Context, projectID, tagDefinitionID string, filter *ListTagAttachmentsFilter) ([]TagAttachment, error) {
	var out []TagAttachment
	var pageToken *string
	for {
		r := c.sdk.TagAPI.ListTagAttachments(ctx, tagDefinitionID)
		if projectID != "" {
			r = r.XProjectId(projectID)
		}
		if filter != nil {
			if filter.TargetType != nil {
				r = r.TargetType(managementv1.TagScope(*filter.TargetType))
			}
			if filter.WarehouseID != nil {
				r = r.WarehouseId(*filter.WarehouseID)
			}
			if filter.Value != nil {
				r = r.Value(*filter.Value)
			}
		}
		if pageToken != nil {
			r = r.PageToken(*pageToken)
		}
		res, resp, err := r.Execute()
		if err != nil {
			return nil, wrapGenErr("list tag attachments", resp, err)
		}
		for _, a := range res.Attachments {
			target, err := tagAttachmentTargetFromSDK(a.Target)
			if err != nil {
				return nil, err
			}
			out = append(out, TagAttachment{
				Target:    target,
				Value:     a.Value.Get(),
				Source:    string(a.Source),
				CreatedAt: formatTime(a.CreatedAt),
				UpdatedAt: formatNullableTime(a.UpdatedAt),
			})
		}
		if next, ok := res.GetNextPageTokenOk(); ok && next != nil && *next != "" {
			pageToken = next
			continue
		}
		break
	}
	return out, nil
}

func buildSetTagRequest(value *string) managementv1.SetTagRequest {
	req := managementv1.NewSetTagRequest()
	if value != nil {
		req.SetValue(*value)
	}
	return *req
}

func wrapTagCall(op string, tag *managementv1.AppliedTag, resp *http.Response, err error) (*AppliedTag, error) {
	if err != nil {
		return nil, wrapGenErr(op, resp, err)
	}
	return appliedTagFromSDK(tag), nil
}

func wrapTagListCall(op string, res *managementv1.ListTagsResponse, resp *http.Response, err error) ([]TargetTag, error) {
	if err != nil {
		return nil, wrapGenErr(op, resp, err)
	}
	return targetTagsFromSDK(res), nil
}

// SetWarehouseTag attaches or updates tagName on warehouseID.
func (c *Client) SetWarehouseTag(ctx context.Context, projectID, warehouseID, tagName string, value *string) (*AppliedTag, error) {
	r := c.sdk.TagAPI.SetWarehouseTag(ctx, warehouseID, tagName).SetTagRequest(buildSetTagRequest(value))
	if projectID != "" {
		r = r.XProjectId(projectID)
	}
	tag, resp, err := r.Execute()
	return wrapTagCall("set warehouse tag", tag, resp, err)
}

// ListWarehouseTags lists tags on warehouseID.
func (c *Client) ListWarehouseTags(ctx context.Context, projectID, warehouseID string, effective bool) ([]TargetTag, error) {
	r := c.sdk.TagAPI.ListWarehouseTags(ctx, warehouseID).Effective(effective)
	if projectID != "" {
		r = r.XProjectId(projectID)
	}
	res, resp, err := r.Execute()
	return wrapTagListCall("list warehouse tags", res, resp, err)
}

// DeleteWarehouseTag removes tagName from warehouseID.
func (c *Client) DeleteWarehouseTag(ctx context.Context, projectID, warehouseID, tagName string) error {
	r := c.sdk.TagAPI.DeleteWarehouseTag(ctx, warehouseID, tagName)
	if projectID != "" {
		r = r.XProjectId(projectID)
	}
	resp, err := r.Execute()
	return wrapGenErr("delete warehouse tag", resp, err)
}

// SetNamespaceTag attaches or updates tagName on namespaceID.
func (c *Client) SetNamespaceTag(ctx context.Context, warehouseID, namespaceID, tagName string, value *string) (*AppliedTag, error) {
	tag, resp, err := c.sdk.TagAPI.SetNamespaceTag(ctx, warehouseID, namespaceID, tagName).SetTagRequest(buildSetTagRequest(value)).Execute()
	return wrapTagCall("set namespace tag", tag, resp, err)
}

// ListNamespaceTags lists tags on namespaceID.
func (c *Client) ListNamespaceTags(ctx context.Context, warehouseID, namespaceID string, effective bool) ([]TargetTag, error) {
	res, resp, err := c.sdk.TagAPI.ListNamespaceTags(ctx, warehouseID, namespaceID).Effective(effective).Execute()
	return wrapTagListCall("list namespace tags", res, resp, err)
}

// DeleteNamespaceTag removes tagName from namespaceID.
func (c *Client) DeleteNamespaceTag(ctx context.Context, warehouseID, namespaceID, tagName string) error {
	resp, err := c.sdk.TagAPI.DeleteNamespaceTag(ctx, warehouseID, namespaceID, tagName).Execute()
	return wrapGenErr("delete namespace tag", resp, err)
}

// SetTableTag attaches or updates tagName on tableID.
func (c *Client) SetTableTag(ctx context.Context, warehouseID, tableID, tagName string, value *string) (*AppliedTag, error) {
	tag, resp, err := c.sdk.TagAPI.SetTableTag(ctx, warehouseID, tableID, tagName).SetTagRequest(buildSetTagRequest(value)).Execute()
	return wrapTagCall("set table tag", tag, resp, err)
}

// ListTableTags lists tags on tableID.
func (c *Client) ListTableTags(ctx context.Context, warehouseID, tableID string, effective bool) ([]TargetTag, error) {
	res, resp, err := c.sdk.TagAPI.ListTableTags(ctx, warehouseID, tableID).Effective(effective).Execute()
	return wrapTagListCall("list table tags", res, resp, err)
}

// DeleteTableTag removes tagName from tableID.
func (c *Client) DeleteTableTag(ctx context.Context, warehouseID, tableID, tagName string) error {
	resp, err := c.sdk.TagAPI.DeleteTableTag(ctx, warehouseID, tableID, tagName).Execute()
	return wrapGenErr("delete table tag", resp, err)
}

// SetViewTag attaches or updates tagName on viewID.
func (c *Client) SetViewTag(ctx context.Context, warehouseID, viewID, tagName string, value *string) (*AppliedTag, error) {
	tag, resp, err := c.sdk.TagAPI.SetViewTag(ctx, warehouseID, viewID, tagName).SetTagRequest(buildSetTagRequest(value)).Execute()
	return wrapTagCall("set view tag", tag, resp, err)
}

// ListViewTags lists tags on viewID.
func (c *Client) ListViewTags(ctx context.Context, warehouseID, viewID string, effective bool) ([]TargetTag, error) {
	res, resp, err := c.sdk.TagAPI.ListViewTags(ctx, warehouseID, viewID).Effective(effective).Execute()
	return wrapTagListCall("list view tags", res, resp, err)
}

// DeleteViewTag removes tagName from viewID.
func (c *Client) DeleteViewTag(ctx context.Context, warehouseID, viewID, tagName string) error {
	resp, err := c.sdk.TagAPI.DeleteViewTag(ctx, warehouseID, viewID, tagName).Execute()
	return wrapGenErr("delete view tag", resp, err)
}

// SetGenericTableTag attaches or updates tagName on genericTableID.
func (c *Client) SetGenericTableTag(ctx context.Context, warehouseID, genericTableID, tagName string, value *string) (*AppliedTag, error) {
	tag, resp, err := c.sdk.TagAPI.SetGenericTableTag(ctx, warehouseID, genericTableID, tagName).SetTagRequest(buildSetTagRequest(value)).Execute()
	return wrapTagCall("set generic table tag", tag, resp, err)
}

// ListGenericTableTags lists tags on genericTableID.
func (c *Client) ListGenericTableTags(ctx context.Context, warehouseID, genericTableID string, effective bool) ([]TargetTag, error) {
	res, resp, err := c.sdk.TagAPI.ListGenericTableTags(ctx, warehouseID, genericTableID).Effective(effective).Execute()
	return wrapTagListCall("list generic table tags", res, resp, err)
}

// DeleteGenericTableTag removes tagName from genericTableID.
func (c *Client) DeleteGenericTableTag(ctx context.Context, warehouseID, genericTableID, tagName string) error {
	resp, err := c.sdk.TagAPI.DeleteGenericTableTag(ctx, warehouseID, genericTableID, tagName).Execute()
	return wrapGenErr("delete generic table tag", resp, err)
}

// SetTableColumnTag attaches or updates tagName on columnName of tableID.
func (c *Client) SetTableColumnTag(ctx context.Context, warehouseID, tableID, columnName, tagName string, value *string) (*AppliedTag, error) {
	tag, resp, err := c.sdk.TagAPI.SetTableColumnTag(ctx, warehouseID, tableID, columnName, tagName).SetTagRequest(buildSetTagRequest(value)).Execute()
	return wrapTagCall("set table column tag", tag, resp, err)
}

// ListTableColumnTags lists tags on columnName of tableID.
func (c *Client) ListTableColumnTags(ctx context.Context, warehouseID, tableID, columnName string, effective bool) ([]TargetTag, error) {
	res, resp, err := c.sdk.TagAPI.ListTableColumnTags(ctx, warehouseID, tableID, columnName).Effective(effective).Execute()
	return wrapTagListCall("list table column tags", res, resp, err)
}

// DeleteTableColumnTag removes tagName from columnName of tableID.
func (c *Client) DeleteTableColumnTag(ctx context.Context, warehouseID, tableID, columnName, tagName string) error {
	resp, err := c.sdk.TagAPI.DeleteTableColumnTag(ctx, warehouseID, tableID, columnName, tagName).Execute()
	return wrapGenErr("delete table column tag", resp, err)
}

// ListAllColumnTags lists all tagged columns of tableID.
func (c *Client) ListAllColumnTags(ctx context.Context, warehouseID, tableID string) ([]ColumnTags, error) {
	res, resp, err := c.sdk.TagAPI.ListColumnTags(ctx, warehouseID, tableID).Execute()
	if err != nil {
		return nil, wrapGenErr("list column tags", resp, err)
	}
	out := make([]ColumnTags, len(res.Columns))
	for i, col := range res.Columns {
		tags := make([]TargetTag, len(col.Tags))
		for j, t := range col.Tags {
			tags[j] = targetTagFromSDK(t)
		}
		out[i] = ColumnTags{
			FieldID: col.FieldId,
			Tags:    tags,
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Tag Assignments

// GetTagAssignments returns every direct grant on tagDefinitionID.
func (c *Client) GetTagAssignments(ctx context.Context, tagDefinitionID string) ([]Assignment, error) {
	res, resp, err := c.sdk.PermissionsOpenfgaAPI.GetTagAssignmentsById(ctx, tagDefinitionID).Execute()
	if err != nil {
		return nil, wrapGenErr("get tag assignments", resp, err)
	}
	return assignmentsFromSDK("tag", res.Assignments)
}

// UpdateTagAssignments applies grant writes and deletes on tagDefinitionID in one request.
func (c *Client) UpdateTagAssignments(ctx context.Context, tagDefinitionID string, writes, deletes []Assignment) error {
	w, d, err := writesAndDeletes[managementv1.TagAssignment](writes, deletes)
	if err != nil {
		return err
	}
	req := managementv1.NewUpdateTagAssignmentsRequest()
	req.Writes = w
	req.Deletes = d
	resp, err := c.sdk.PermissionsOpenfgaAPI.UpdateTagAssignmentsById(ctx, tagDefinitionID).UpdateTagAssignmentsRequest(*req).Execute()
	return wrapGenErr("update tag assignments", resp, err)
}
