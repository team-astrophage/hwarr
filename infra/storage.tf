############################
# ElastiCache Redis — single-node t4g.micro
# (Serverless was overkill for ~30 users and cost ~$70+/mo idle)
############################

resource "aws_elasticache_subnet_group" "redis" {
  name       = "${local.name_prefix}-redis-subnet"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "${local.name_prefix}-redis-subnet"
  }
}

resource "aws_elasticache_replication_group" "redis" {
  # NOTE: Serverless cache 와 동일 이름 공간을 공유하므로, 마이그레이션 중에는
  # 옛 serverless ('${local.name_prefix}-redis') 와 충돌하지 않도록 -rg 접미사를 붙임.
  # 옛 serverless 가 destroy 된 뒤에도 이름 변경은 destructive 라 그대로 유지.
  replication_group_id = "${local.name_prefix}-redis-rg"
  description          = "Redis (single-node ${var.redis_node_type}) for ${local.name_prefix}"

  engine         = "redis"
  engine_version = "7.1"
  node_type      = var.redis_node_type
  port           = 6379

  # Single node — no replicas, no automatic failover
  num_cache_clusters         = 1
  automatic_failover_enabled = false
  multi_az_enabled           = false

  parameter_group_name = "default.redis7"
  subnet_group_name    = aws_elasticache_subnet_group.redis.name
  security_group_ids   = [aws_security_group.redis.id]

  # TLS in-transit (matches existing rediss:// scheme in REDIS_URL).
  # No auth_token — VPC-internal only, SG locked to ECS tasks.
  transit_encryption_enabled = true
  at_rest_encryption_enabled = true

  # Daily snapshots (cumulative stats, etc.)
  snapshot_retention_limit = 7
  snapshot_window          = "19:00-20:00" # 04:00-05:00 KST

  # Seed initial data from RDB exported by the prior Serverless cache.
  # Only consulted on first creation; safe to leave empty after migration.
  snapshot_arns = var.redis_seed_snapshot_arn == "" ? null : [var.redis_seed_snapshot_arn]

  apply_immediately = true

  lifecycle {
    # snapshot_arns is only used on initial create; ignore drift afterward
    ignore_changes = [snapshot_arns]
  }

  tags = {
    Name = "${local.name_prefix}-redis"
  }
}

############################
# S3 — Frontend Static Assets
############################

resource "aws_s3_bucket" "frontend" {
  bucket        = var.frontend_bucket_name
  force_destroy = true # hackathon — easy teardown

  tags = {
    Name = "${local.name_prefix}-frontend"
  }
}

resource "aws_s3_bucket_versioning" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

############################
# CloudFront OAC — S3 Access Control
############################

resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${local.name_prefix}-oac"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

############################
# CloudFront Distribution
############################

resource "aws_cloudfront_distribution" "frontend" {
  enabled             = true
  default_root_object = "index.html"
  comment             = "${local.name_prefix} — single-domain CDN (FE + BE)"

  # ── Origin 1: S3 (frontend static files) ──
  origin {
    domain_name              = aws_s3_bucket.frontend.bucket_regional_domain_name
    origin_id                = "s3-frontend"
    origin_access_control_id = aws_cloudfront_origin_access_control.frontend.id
  }

  # ── Origin 2: ALB (backend API + WebSocket) ──
  origin {
    domain_name = aws_lb.main.dns_name
    origin_id   = "alb-api"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "http-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  # ── Default behavior: S3 static files ──
  default_cache_behavior {
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "s3-frontend"
    viewer_protocol_policy = "redirect-to-https"
    compress               = true

    forwarded_values {
      query_string = false
      cookies {
        forward = "none"
      }
    }

    min_ttl     = 0
    default_ttl = 86400
    max_ttl     = 31536000
  }

  # ── /api/* → ALB (REST API, no caching) ──
  ordered_cache_behavior {
    path_pattern           = "/api/*"
    allowed_methods        = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "alb-api"
    viewer_protocol_policy = "redirect-to-https"

    forwarded_values {
      query_string = true
      # User-Agent forwarded so the backend can log the real viewer UA
      # (CloudFront otherwise replaces it with "Amazon CloudFront").
      # Caching on /api/* is fully disabled (ttl=0) so this doesn't hurt hit rate.
      headers = [
        "Origin",
        "Access-Control-Request-Headers",
        "Access-Control-Request-Method",
        "User-Agent",
      ]
      cookies {
        forward = "none"
      }
    }

    min_ttl     = 0
    default_ttl = 0
    max_ttl     = 0
  }

  # ── /socket.io/* → ALB (WebSocket, no caching, all headers) ──
  ordered_cache_behavior {
    path_pattern           = "/socket.io/*"
    allowed_methods        = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "alb-api"
    viewer_protocol_policy = "redirect-to-https"

    forwarded_values {
      query_string = true
      headers      = ["*"] # WebSocket upgrade requires all headers
      cookies {
        forward = "all" # Socket.IO polling needs cookies for sticky sessions
      }
    }

    min_ttl     = 0
    default_ttl = 0
    max_ttl     = 0
  }

  # ── /health → ALB (health check proxy, optional) ──
  ordered_cache_behavior {
    path_pattern           = "/health"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "alb-api"
    viewer_protocol_policy = "redirect-to-https"

    forwarded_values {
      query_string = false
      cookies {
        forward = "none"
      }
    }

    min_ttl     = 0
    default_ttl = 0
    max_ttl     = 0
  }

  # ── SPA routing: 403/404 → index.html ──
  custom_error_response {
    error_code         = 403
    response_code      = 200
    response_page_path = "/index.html"
  }

  custom_error_response {
    error_code         = 404
    response_code      = 200
    response_page_path = "/index.html"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  aliases = [var.domain_name, "www.${var.domain_name}"]

  viewer_certificate {
    acm_certificate_arn      = aws_acm_certificate_validation.cloudfront.certificate_arn
    ssl_support_method       = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }

  tags = {
    Name = "${local.name_prefix}-cdn"
  }
}

############################
# S3 Bucket Policy — CloudFront OAC only
############################

resource "aws_s3_bucket_policy" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowCloudFrontOAC"
        Effect    = "Allow"
        Principal = { Service = "cloudfront.amazonaws.com" }
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.frontend.arn}/*"
        Condition = {
          StringEquals = {
            "AWS:SourceArn" = aws_cloudfront_distribution.frontend.arn
          }
        }
      }
    ]
  })
}
