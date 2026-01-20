$outFile = "combined-code.txt"
$root = Get-Location
$extensions = @(".go", ".mod", ".rego", ".md", ".yml", ".yaml", ".ps1", ".work")

# Initialize output file
Set-Content -Path $outFile -Value "Combined Code Export - $(Get-Date)`n"

# Get all files recursively
Get-ChildItem -Path $root -Recurse -File | Where-Object { 
    $ext = $_.Extension
    # Include specific extensions
    ($extensions -contains $ext) -or ($_.Name -eq "Dockerfile")
} | Where-Object {
    # Exclude the output file itself and git directory
    $_.Name -ne $outFile -and $_.FullName -notmatch "\\.git\\"
} | ForEach-Object {
    $relativePath = $_.FullName.Substring($root.Path.Length + 1)
    
    Write-Host "Processing: $relativePath"
    
    Add-Content -Path $outFile -Value "`n================================================================================"
    Add-Content -Path $outFile -Value "FILE: $relativePath"
    Add-Content -Path $outFile -Value "================================================================================`n"
    
    Try {
        $content = Get-Content -Path $_.FullName -Raw -ErrorAction Stop
        Add-Content -Path $outFile -Value $content
    } Catch {
        Add-Content -Path $outFile -Value "[Error reading file: $($_.Exception.Message)]"
    }
}

Write-Host "`nSuccessfully created $outFile"
