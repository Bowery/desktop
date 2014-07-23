//
//  BoweryAppDelegate.m
//  BoweryMenuApp
//
//  Created by Steve Kaliski on 7/10/14.
//  Copyright (c) 2014 Bowery. All rights reserved.
//

#import "BoweryAppDelegate.h"
#import <WebKit/WebKit.h>

@implementation BoweryAppDelegate

@synthesize refreshBtn = _refreshBtn;

NSTask *task;

- (void)applicationDidFinishLaunching:(NSNotification *)notification
{
    task = [[NSTask alloc] init];

    // find the client
    NSString* client = [[NSBundle mainBundle] pathForResource:@"Bowery/client" ofType:@""];
    [task setLaunchPath:client];
    [task launch];

    [self.webView setDrawsBackground:NO];
    
	NSURLRequest *request = [NSURLRequest requestWithURL:[NSURL URLWithString:@"http://0.0.0.0:32055/login"]];
	[self.webView.mainFrame loadRequest:request];
}

- (IBAction)doSomething:(id)sender {
    
    // Load Google and change button accordingly
    if ([[_refreshBtn title] isEqualTo:@"Load Yahoo"]) {
        [_refreshBtn setTitle:@"Load Google"];
        NSURLRequest *request = [NSURLRequest requestWithURL:[NSURL URLWithString:@"http://0.0.0.0:32055/login"]];
        [self.webView.mainFrame loadRequest:request];
        
    }
    
    // Load Yahoo and change button accordingly
    else {
        [_refreshBtn setTitle:@"Load Yahoo"];
        NSURLRequest *request = [NSURLRequest requestWithURL:[NSURL URLWithString:@"http://0.0.0.0:32055"]];
        [self.webView.mainFrame loadRequest:request];
        
    }
    
}

-(BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
    [task terminate];
    return YES;
}

@end
