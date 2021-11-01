set FileName=DownloadEmailsAttachments.exe

go build main.go
del /Q %FileName%
ren main.exe %FileName%
copy %FileName% DownloadEmailsAttachments_ready\%FileName%
