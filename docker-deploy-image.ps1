# Build and optionally push the kanban233 Docker image.
#
# Examples:
#   .\docker-deploy-image.ps1
#   .\docker-deploy-image.ps1 -Registry registry.example.com/team -Tag 1.0.0 -Push
#   .\docker-deploy-image.ps1 -Registry registry.example.com/team -Tag 1.0.0 -Push -PushLatest

param(
    [string]$Registry = "",
    [string]$ImageName = "kanban233",
    [string]$Tag = "latest",
    [switch]$Push,
    [switch]$PushLatest
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

function Get-FullImageName {
    param([string]$TagSuffix)
    if ($Registry) {
        return "$Registry/$ImageName`:$TagSuffix"
    }
    return "$ImageName`:$TagSuffix"
}

$imageTag = Get-FullImageName -TagSuffix $Tag

Write-Host "[docker] building $imageTag ..."
docker build -t $imageTag .
if ($LASTEXITCODE -ne 0) {
    throw "docker build failed with exit code $LASTEXITCODE"
}

Write-Host "[docker] built $imageTag"

if (-not $Push) {
    Write-Host "[docker] skip push (use -Push to publish)"
    exit 0
}

Write-Host "[docker] pushing $imageTag ..."
docker push $imageTag
if ($LASTEXITCODE -ne 0) {
    throw "docker push failed with exit code $LASTEXITCODE"
}

if ($PushLatest -and $Tag -ne "latest") {
    $latestTag = Get-FullImageName -TagSuffix "latest"
    Write-Host "[docker] tagging $latestTag ..."
    docker tag $imageTag $latestTag
    if ($LASTEXITCODE -ne 0) {
        throw "docker tag failed with exit code $LASTEXITCODE"
    }
    Write-Host "[docker] pushing $latestTag ..."
    docker push $latestTag
    if ($LASTEXITCODE -ne 0) {
        throw "docker push failed with exit code $LASTEXITCODE"
    }
}

Write-Host "[docker] done"
