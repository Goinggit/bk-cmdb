/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.,
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the ",License",); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an ",AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package extensions

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"configcenter/src/ac/meta"
	"configcenter/src/common/blog"
	"configcenter/src/common/metadata"
	"configcenter/src/common/util"
)

/*
 * service template
 */

func (am *AuthManager) collectServiceTemplateByIDs(ctx context.Context, header http.Header, templateIDs ...int64) ([]metadata.ServiceTemplate, error) {
	rid := util.ExtractRequestIDFromContext(ctx)

	// unique ids so that we can be aware of invalid id if query result length not equal ids's length
	templateIDs = util.IntArrayUnique(templateIDs)
	option := &metadata.ListServiceTemplateOption{
		ServiceTemplateIDs: templateIDs,
	}
	result, err := am.clientSet.CoreService().Process().ListServiceTemplates(ctx, header, option)
	if err != nil {
		blog.V(3).Infof("list service templates by id failed, templateIDs: %+v, err: %+v, rid: %s", templateIDs, err, rid)
		return nil, fmt.Errorf("list service templates by id failed, err: %+v", err)
	}

	return result.Info, nil
}

func (am *AuthManager) extractBusinessIDFromServiceTemplate(templates ...metadata.ServiceTemplate) (int64, error) {
	var businessID int64
	for idx, template := range templates {
		bizID := template.BizID
		// we should ignore metadata.LabelBusinessID field not found error
		if idx > 0 && bizID != businessID {
			return 0, fmt.Errorf("get multiple business ID from service templates")
		}
		businessID = bizID
	}
	return businessID, nil
}

func (am *AuthManager) MakeResourcesByServiceTemplate(header http.Header, action meta.Action, businessID int64, templates ...metadata.ServiceTemplate) []meta.ResourceAttribute {
	resources := make([]meta.ResourceAttribute, 0)
	for _, template := range templates {
		resource := meta.ResourceAttribute{
			Basic: meta.Basic{
				Action:     action,
				Type:       meta.ProcessServiceTemplate,
				Name:       template.Name,
				InstanceID: template.ID,
			},
			SupplierAccount: util.GetOwnerID(header),
			BusinessID:      businessID,
		}

		resources = append(resources, resource)
	}
	return resources
}

func (am *AuthManager) AuthorizeByServiceTemplateID(ctx context.Context, header http.Header, action meta.Action, ids ...int64) error {
	if !am.Enabled() {
		return nil
	}

	if len(ids) == 0 {
		return nil
	}

	templates, err := am.collectServiceTemplateByIDs(ctx, header, ids...)
	if err != nil {
		return fmt.Errorf("get service templates by id failed, err: %+v", err)
	}
	return am.AuthorizeByServiceTemplates(ctx, header, action, templates...)
}

func (am *AuthManager) GenServiceTemplateNoPermissionResp() *metadata.BaseResp {
	// TODO implement this
	resp := metadata.NewNoPermissionResp([]metadata.Permission{})
	return &resp
}

func (am *AuthManager) AuthorizeByServiceTemplates(ctx context.Context, header http.Header, action meta.Action, templates ...metadata.ServiceTemplate) error {
	if !am.Enabled() {
		return nil
	}

	if len(templates) == 0 {
		return nil
	}
	// extract business id
	bizID, err := am.extractBusinessIDFromServiceTemplate(templates...)
	if err != nil {
		return fmt.Errorf("authorize service templates failed, extract business id from service templates failed, err: %+v", err)
	}

	// make auth resources
	resources := am.MakeResourcesByServiceTemplate(header, action, bizID, templates...)

	return am.authorize(ctx, header, bizID, resources...)
}

func (am *AuthManager) ListAuthorizedServiceTemplateIDs(ctx context.Context, header http.Header, bizID int64) ([]int64, error) {
	rid := util.ExtractRequestIDFromContext(ctx)
	input := meta.ListAuthorizedResourcesParam{
		Username:     util.GetUser(header),
		BizID:        0,
		ResourceType: meta.ProcessServiceTemplate,
		Action:       meta.FindMany,
	}
	resources, err := am.clientSet.AuthServer().ListAuthorizedResources(ctx, header, input)
	if err != nil {
		blog.Errorf("list authorized service template from iam failed, err: %+v, rid: %s", err, rid)
		return nil, err
	}
	ids := make([]int64, 0)
	for _, item := range resources {
		for _, resource := range item {
			id, err := strconv.ParseInt(resource.ResourceID, 10, 64)
			if err != nil {
				blog.Errorf("list authorized service template from iam failed, err: %+v, rid: %s", err, rid)
				return nil, fmt.Errorf("parse resource id into int64 failed, err: %+v", err)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}
