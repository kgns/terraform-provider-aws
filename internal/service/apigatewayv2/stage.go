// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package apigatewayv2

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	awstypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
	"github.com/hashicorp/terraform-provider-aws/names"
)

const (
	defaultStageName = "$default"
)

// @SDKResource("aws_apigatewayv2_stage", name="Stage")
// @Tags(identifierAttribute="arn")
func ResourceStage() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceStageCreate,
		ReadWithoutTimeout:   resourceStageRead,
		UpdateWithoutTimeout: resourceStageUpdate,
		DeleteWithoutTimeout: resourceStageDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceStageImport,
		},

		Schema: map[string]*schema.Schema{
			"access_log_settings": {
				Type:     schema.TypeList,
				Optional: true,
				MinItems: 0,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"destination_arn": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: verify.ValidARN,
						},
						"format": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"api_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"auto_deploy": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"client_certificate_id": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"default_route_settings": {
				Type:             schema.TypeList,
				Optional:         true,
				MinItems:         0,
				MaxItems:         1,
				DiffSuppressFunc: verify.SuppressMissingOptionalConfigurationBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"data_trace_enabled": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"detailed_metrics_enabled": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"logging_level": {
							Type:         schema.TypeString,
							Optional:     true,
							Computed:     true,
							ValidateFunc: validation.StringInSlice(enum.Slice(awstypes.LoggingLevelError, awstypes.LoggingLevelInfo, awstypes.LoggingLevelOff), false),
						},
						"throttling_burst_limit": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"throttling_rate_limit": {
							Type:     schema.TypeFloat,
							Optional: true,
						},
					},
				},
			},
			"deployment_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"description": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(0, 1024),
			},
			"execution_arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"invoke_url": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(1, 128),
			},
			"route_settings": {
				Type:     schema.TypeSet,
				Optional: true,
				MinItems: 0,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"data_trace_enabled": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"detailed_metrics_enabled": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"logging_level": {
							Type:         schema.TypeString,
							Optional:     true,
							Computed:     true,
							ValidateFunc: validation.StringInSlice(enum.Slice(awstypes.LoggingLevelError, awstypes.LoggingLevelInfo, awstypes.LoggingLevelOff), false),
						},
						"route_key": {
							Type:     schema.TypeString,
							Required: true,
						},
						"throttling_burst_limit": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"throttling_rate_limit": {
							Type:     schema.TypeFloat,
							Optional: true,
						},
					},
				},
			},
			"stage_variables": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			names.AttrTags:    tftags.TagsSchema(),
			names.AttrTagsAll: tftags.TagsSchemaComputed(),
		},

		CustomizeDiff: verify.SetTagsDiff,
	}
}

func resourceStageCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).APIGatewayV2Client(ctx)

	apiId := d.Get("api_id").(string)

	apiOutput, err := conn.GetApi(ctx, &apigatewayv2.GetApiInput{
		ApiId: aws.String(apiId),
	})
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading API Gateway v2 API (%s): %s", apiId, err)
	}

	protocolType := string(apiOutput.ProtocolType)

	req := &apigatewayv2.CreateStageInput{
		ApiId:      aws.String(apiId),
		AutoDeploy: aws.Bool(d.Get("auto_deploy").(bool)),
		StageName:  aws.String(d.Get("name").(string)),
		Tags:       getTagsIn(ctx),
	}
	if v, ok := d.GetOk("access_log_settings"); ok {
		req.AccessLogSettings = expandAccessLogSettings(v.([]interface{}))
	}
	if v, ok := d.GetOk("client_certificate_id"); ok {
		req.ClientCertificateId = aws.String(v.(string))
	}
	if v, ok := d.GetOk("default_route_settings"); ok {
		req.DefaultRouteSettings = expandDefaultRouteSettings(v.([]interface{}), protocolType)
	}
	if v, ok := d.GetOk("deployment_id"); ok {
		req.DeploymentId = aws.String(v.(string))
	}
	if v, ok := d.GetOk("description"); ok {
		req.Description = aws.String(v.(string))
	}
	if v, ok := d.GetOk("route_settings"); ok {
		req.RouteSettings = expandRouteSettings(v.(*schema.Set).List(), protocolType)
	}
	if v, ok := d.GetOk("stage_variables"); ok {
		req.StageVariables = flex.ExpandStringValueMap(v.(map[string]interface{}))
	}

	log.Printf("[DEBUG] Creating API Gateway v2 stage: %+v", req)
	resp, err := conn.CreateStage(ctx, req)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating API Gateway v2 stage: %s", err)
	}

	d.SetId(aws.ToString(resp.StageName))

	return append(diags, resourceStageRead(ctx, d, meta)...)
}

func resourceStageRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).APIGatewayV2Client(ctx)

	apiId := d.Get("api_id").(string)
	resp, err := conn.GetStage(ctx, &apigatewayv2.GetStageInput{
		ApiId:     aws.String(apiId),
		StageName: aws.String(d.Id()),
	})
	if errs.IsA[*awstypes.NotFoundException](err) && !d.IsNewResource() {
		log.Printf("[WARN] API Gateway v2 stage (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading API Gateway v2 stage (%s): %s", d.Id(), err)
	}

	stageName := aws.ToString(resp.StageName)
	err = d.Set("access_log_settings", flattenAccessLogSettings(resp.AccessLogSettings))
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "setting access_log_settings: %s", err)
	}
	region := meta.(*conns.AWSClient).Region
	resourceArn := arn.ARN{
		Partition: meta.(*conns.AWSClient).Partition,
		Service:   "apigateway",
		Region:    region,
		Resource:  fmt.Sprintf("/apis/%s/stages/%s", apiId, stageName),
	}.String()
	d.Set("arn", resourceArn)
	d.Set("auto_deploy", resp.AutoDeploy)
	d.Set("client_certificate_id", resp.ClientCertificateId)
	err = d.Set("default_route_settings", flattenDefaultRouteSettings(resp.DefaultRouteSettings))
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "setting default_route_settings: %s", err)
	}
	d.Set("deployment_id", resp.DeploymentId)
	d.Set("description", resp.Description)
	executionArn := arn.ARN{
		Partition: meta.(*conns.AWSClient).Partition,
		Service:   "execute-api",
		Region:    region,
		AccountID: meta.(*conns.AWSClient).AccountID,
		Resource:  fmt.Sprintf("%s/%s", apiId, stageName),
	}.String()
	d.Set("execution_arn", executionArn)
	d.Set("name", stageName)
	err = d.Set("route_settings", flattenRouteSettings(resp.RouteSettings))
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "setting route_settings: %s", err)
	}
	err = d.Set("stage_variables", resp.StageVariables)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "setting stage_variables: %s", err)
	}

	setTagsOut(ctx, resp.Tags)

	apiOutput, err := conn.GetApi(ctx, &apigatewayv2.GetApiInput{
		ApiId: aws.String(apiId),
	})
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading API Gateway v2 API (%s): %s", apiId, err)
	}

	switch apiOutput.ProtocolType {
	case awstypes.ProtocolTypeWebsocket:
		d.Set("invoke_url", fmt.Sprintf("wss://%s.execute-api.%s.amazonaws.com/%s", apiId, region, stageName))
	case awstypes.ProtocolTypeHttp:
		if stageName == defaultStageName {
			d.Set("invoke_url", fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/", apiId, region))
		} else {
			d.Set("invoke_url", fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/%s", apiId, region, stageName))
		}
	}

	return diags
}

func resourceStageUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).APIGatewayV2Client(ctx)

	if d.HasChanges("access_log_settings", "auto_deploy", "client_certificate_id",
		"default_route_settings", "deployment_id", "description",
		"route_settings", "stage_variables") {
		apiId := d.Get("api_id").(string)

		apiOutput, err := conn.GetApi(ctx, &apigatewayv2.GetApiInput{
			ApiId: aws.String(apiId),
		})
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "reading API Gateway v2 API (%s): %s", apiId, err)
		}

		protocolType := string(apiOutput.ProtocolType)

		req := &apigatewayv2.UpdateStageInput{
			ApiId:     aws.String(apiId),
			StageName: aws.String(d.Id()),
		}
		if d.HasChange("access_log_settings") {
			req.AccessLogSettings = expandAccessLogSettings(d.Get("access_log_settings").([]interface{}))
		}
		if d.HasChange("auto_deploy") {
			req.AutoDeploy = aws.Bool(d.Get("auto_deploy").(bool))
		}
		if d.HasChange("client_certificate_id") {
			req.ClientCertificateId = aws.String(d.Get("client_certificate_id").(string))
		}
		if d.HasChange("default_route_settings") {
			req.DefaultRouteSettings = expandDefaultRouteSettings(d.Get("default_route_settings").([]interface{}), protocolType)
		}
		if d.HasChange("deployment_id") {
			req.DeploymentId = aws.String(d.Get("deployment_id").(string))
		}
		if d.HasChange("description") {
			req.Description = aws.String(d.Get("description").(string))
		}
		if d.HasChange("route_settings") {
			o, n := d.GetChange("route_settings")
			os := o.(*schema.Set)
			ns := n.(*schema.Set)

			for _, vRouteSetting := range os.Difference(ns).List() {
				routeKey := vRouteSetting.(map[string]interface{})["route_key"].(string)

				log.Printf("[DEBUG] Deleting API Gateway v2 stage (%s) route settings (%s)", d.Id(), routeKey)
				_, err := conn.DeleteRouteSettings(ctx, &apigatewayv2.DeleteRouteSettingsInput{
					ApiId:     aws.String(d.Get("api_id").(string)),
					RouteKey:  aws.String(routeKey),
					StageName: aws.String(d.Id()),
				})
				if errs.IsA[*awstypes.NotFoundException](err) {
					continue
				}
				if err != nil {
					return sdkdiag.AppendErrorf(diags, "deleting API Gateway v2 stage (%s) route settings (%s): %s", d.Id(), routeKey, err)
				}
			}

			req.RouteSettings = expandRouteSettings(ns.List(), protocolType)
		}
		if d.HasChange("stage_variables") {
			o, n := d.GetChange("stage_variables")
			add, del, _ := flex.DiffStringValueMaps(o.(map[string]interface{}), n.(map[string]interface{}))
			// Variables are removed by setting the associated value to "".
			for k := range del {
				del[k] = ""
			}
			variables := del
			for k, v := range add {
				variables[k] = v
			}
			req.StageVariables = variables
		}

		log.Printf("[DEBUG] Updating API Gateway v2 stage: %+v", req)
		_, err = conn.UpdateStage(ctx, req)
		if err != nil {
			return sdkdiag.AppendErrorf(diags, "updating API Gateway v2 stage (%s): %s", d.Id(), err)
		}
	}

	return append(diags, resourceStageRead(ctx, d, meta)...)
}

func resourceStageDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).APIGatewayV2Client(ctx)

	log.Printf("[DEBUG] Deleting API Gateway v2 stage (%s)", d.Id())
	_, err := conn.DeleteStage(ctx, &apigatewayv2.DeleteStageInput{
		ApiId:     aws.String(d.Get("api_id").(string)),
		StageName: aws.String(d.Id()),
	})
	if errs.IsA[*awstypes.NotFoundException](err) {
		return diags
	}
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting API Gateway v2 stage (%s): %s", d.Id(), err)
	}

	return diags
}

func resourceStageImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	parts := strings.Split(d.Id(), "/")
	if len(parts) != 2 {
		return []*schema.ResourceData{}, fmt.Errorf("wrong format of import ID (%s), use: 'api-id/stage-name'", d.Id())
	}

	apiId := parts[0]
	stageName := parts[1]

	conn := meta.(*conns.AWSClient).APIGatewayV2Client(ctx)

	resp, err := conn.GetStage(ctx, &apigatewayv2.GetStageInput{
		ApiId:     aws.String(apiId),
		StageName: aws.String(stageName),
	})
	if err != nil {
		return nil, err
	}

	if aws.ToBool(resp.ApiGatewayManaged) {
		return nil, fmt.Errorf("API Gateway v2 stage (%s) was created via quick create", stageName)
	}

	d.SetId(stageName)
	d.Set("api_id", apiId)

	return []*schema.ResourceData{d}, nil
}

func expandAccessLogSettings(vSettings []interface{}) *awstypes.AccessLogSettings {
	settings := &awstypes.AccessLogSettings{}

	if len(vSettings) == 0 || vSettings[0] == nil {
		return settings
	}
	mSettings := vSettings[0].(map[string]interface{})

	if vDestinationArn, ok := mSettings["destination_arn"].(string); ok && vDestinationArn != "" {
		settings.DestinationArn = aws.String(vDestinationArn)
	}
	if vFormat, ok := mSettings["format"].(string); ok && vFormat != "" {
		settings.Format = aws.String(vFormat)
	}

	return settings
}

func flattenAccessLogSettings(settings *awstypes.AccessLogSettings) []interface{} {
	if settings == nil {
		return []interface{}{}
	}

	return []interface{}{map[string]interface{}{
		"destination_arn": aws.ToString(settings.DestinationArn),
		"format":          aws.ToString(settings.Format),
	}}
}

func expandDefaultRouteSettings(vSettings []interface{}, protocolType string) *awstypes.RouteSettings {
	routeSettings := &awstypes.RouteSettings{}

	if len(vSettings) == 0 || vSettings[0] == nil {
		return routeSettings
	}
	mSettings := vSettings[0].(map[string]interface{})

	if vDataTraceEnabled, ok := mSettings["data_trace_enabled"].(bool); ok && protocolType == string(awstypes.ProtocolTypeWebsocket) {
		routeSettings.DataTraceEnabled = aws.Bool(vDataTraceEnabled)
	}
	if vDetailedMetricsEnabled, ok := mSettings["detailed_metrics_enabled"].(bool); ok {
		routeSettings.DetailedMetricsEnabled = aws.Bool(vDetailedMetricsEnabled)
	}
	if vLoggingLevel, ok := mSettings["logging_level"].(string); ok && vLoggingLevel != "" && protocolType == string(awstypes.ProtocolTypeWebsocket) {
		routeSettings.LoggingLevel = awstypes.LoggingLevel(vLoggingLevel)
	}
	if vThrottlingBurstLimit, ok := mSettings["throttling_burst_limit"].(int); ok {
		routeSettings.ThrottlingBurstLimit = aws.Int32(int32(vThrottlingBurstLimit))
	}
	if vThrottlingRateLimit, ok := mSettings["throttling_rate_limit"].(float64); ok {
		routeSettings.ThrottlingRateLimit = aws.Float64(vThrottlingRateLimit)
	}

	return routeSettings
}

func flattenDefaultRouteSettings(routeSettings *awstypes.RouteSettings) []interface{} {
	if routeSettings == nil {
		return []interface{}{}
	}

	return []interface{}{map[string]interface{}{
		"data_trace_enabled":       aws.ToBool(routeSettings.DataTraceEnabled),
		"detailed_metrics_enabled": aws.ToBool(routeSettings.DetailedMetricsEnabled),
		"logging_level":            string(routeSettings.LoggingLevel),
		"throttling_burst_limit":   int(aws.ToInt32(routeSettings.ThrottlingBurstLimit)),
		"throttling_rate_limit":    aws.ToFloat64(routeSettings.ThrottlingRateLimit),
	}}
}

func expandRouteSettings(vSettings []interface{}, protocolType string) map[string]awstypes.RouteSettings {
	settings := map[string]awstypes.RouteSettings{}

	for _, v := range vSettings {
		routeSettings := awstypes.RouteSettings{}

		mSettings := v.(map[string]interface{})

		if v, ok := mSettings["data_trace_enabled"].(bool); ok && protocolType == string(awstypes.ProtocolTypeWebsocket) {
			routeSettings.DataTraceEnabled = aws.Bool(v)
		}
		if v, ok := mSettings["detailed_metrics_enabled"].(bool); ok {
			routeSettings.DetailedMetricsEnabled = aws.Bool(v)
		}
		if v, ok := mSettings["logging_level"].(string); ok && v != "" && protocolType == string(awstypes.ProtocolTypeWebsocket) {
			routeSettings.LoggingLevel = awstypes.LoggingLevel(v)
		}
		if v, ok := mSettings["throttling_burst_limit"].(int); ok {
			routeSettings.ThrottlingBurstLimit = aws.Int32(int32(v))
		}
		if v, ok := mSettings["throttling_rate_limit"].(float64); ok {
			routeSettings.ThrottlingRateLimit = aws.Float64(v)
		}

		settings[mSettings["route_key"].(string)] = routeSettings
	}

	return settings
}

func flattenRouteSettings(settings map[string]awstypes.RouteSettings) []interface{} {
	vSettings := []interface{}{}

	for k, routeSetting := range settings {
		vSettings = append(vSettings, map[string]interface{}{
			"data_trace_enabled":       aws.ToBool(routeSetting.DataTraceEnabled),
			"detailed_metrics_enabled": aws.ToBool(routeSetting.DetailedMetricsEnabled),
			"logging_level":            awstypes.LoggingLevel(routeSetting.LoggingLevel),
			"route_key":                k,
			"throttling_burst_limit":   int(aws.ToInt32(routeSetting.ThrottlingBurstLimit)),
			"throttling_rate_limit":    aws.ToFloat64(routeSetting.ThrottlingRateLimit),
		})
	}

	return vSettings
}
